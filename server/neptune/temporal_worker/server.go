package temporal_worker

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/cactus/go-statsd-client/statsd"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server"
	"github.com/runatlantis/atlantis/server/controllers/templates"
	cfgParser "github.com/runatlantis/atlantis/server/core/config"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/uber-go/tally/v4"
	tallystatsd "github.com/uber-go/tally/v4/statsd"
	"github.com/urfave/cli"
	"github.com/urfave/negroni"
	"go.temporal.io/sdk/client"
	temporal_tally "go.temporal.io/sdk/contrib/tally"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

const AtlantisNamespace = "atlantis"

type HealthStatus int64

const (
	HEALTHY HealthStatus = iota
	UNHEALTHY
)

type HealthResponse struct {
	Status string `json:"status"`
}

// Config is TemporalWorker specific user config
type Config struct {
	atlantisURL      *url.URL
	atlantisVersion  string
	ctxLogger        logging.Logger
	healthStatus     int32
	metrics          valid.Metrics
	sslCertFile      string
	sslKeyFile       string
	statsNamespace   string
	temporalHostPort string
	scope            tally.Scope
	closer           io.Closer
}

func NewConfig(atlantisURL, atlantisURLFlag, atlantisVersion string, logLevel logging.LogLevel, repoConfig string, sslCertFile, sslKeyFile, statsNamespace, temporalHostPort string) (*Config, error) {
	parsedURL, err := server.ParseAtlantisURL(atlantisURL)
	if err != nil {
		return nil, errors.Wrapf(err,
			"parsing --%s flag %q", atlantisURLFlag, atlantisURL)
	}
	ctxLogger, err := logging.NewLoggerFromLevel(logLevel)
	if err != nil {
		return nil, err
	}
	globalCfg := valid.NewGlobalCfg()
	validator := &cfgParser.ParserValidator{}
	if repoConfig != "" {
		globalCfg, err = validator.ParseGlobalCfg(repoConfig, globalCfg)
		if err != nil {
			return nil, errors.Wrapf(err, "parsing %s file", repoConfig)
		}
	}
	reporter, err := newReporter(globalCfg.Metrics)
	if err != nil {
		return nil, errors.Wrap(err, "creating stats reporter")
	}
	opts := tally.ScopeOptions{
		Prefix:   "atlantis",
		Reporter: reporter,
	}
	// TODO: upgrade server/metrics dir to use tally v4 and be compatible with temporal
	scope, closer := tally.NewRootScope(opts, time.Second)

	config := &Config{
		atlantisURL:      parsedURL,
		atlantisVersion:  atlantisVersion,
		ctxLogger:        ctxLogger,
		scope:            scope,
		closer:           closer,
		sslCertFile:      sslCertFile,
		sslKeyFile:       sslKeyFile,
		statsNamespace:   statsNamespace,
		temporalHostPort: temporalHostPort,
	}
	return config, nil
}

func newReporter(cfg valid.Metrics) (tally.StatsReporter, error) {
	if cfg.Statsd == nil {
		return nil, nil
	}

	statsdCfg := cfg.Statsd
	client, err := statsd.NewClientWithConfig(&statsd.ClientConfig{
		Address: strings.Join([]string{statsdCfg.Host, statsdCfg.Port}, ":"),
	})

	if err != nil {
		return nil, errors.Wrap(err, "initializing statsd client")
	}

	return tallystatsd.NewReporter(client, tallystatsd.Options{}), nil
}

func (c *Config) buildTemporalClient() (client.Client, error) {
	certs, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}
	connectionOptions := client.ConnectionOptions{
		TLS: &tls.Config{
			RootCAs:    certs,
			MinVersion: tls.VersionTLS12,
		},
	}
	clientOptions := client.Options{
		Namespace:         AtlantisNamespace,
		ConnectionOptions: connectionOptions,
		MetricsHandler:    temporal_tally.NewMetricsHandler(c.scope),
	}
	if c.temporalHostPort != "" {
		clientOptions.HostPort = c.temporalHostPort
	}
	return client.Dial(clientOptions)
}

func (c *Config) Index(w http.ResponseWriter, _ *http.Request) {
	err := templates.TemporalWorkerIndexTemplate.Execute(w, templates.IndexData{
		AtlantisVersion: c.atlantisVersion,
		CleanedBasePath: c.atlantisURL.Path,
	})
	if err != nil {
		c.ctxLogger.Error(err.Error())
	}
}

type Server struct {
	Logger         logging.Logger
	SSLCertFile    string
	SSLKeyFile     string
	Router         *mux.Router
	LyftMode       server.Mode
	Port           int
	StatsCloser    io.Closer
	TemporalClient client.Client
	HealthStatus   int32
}

// TODO: as more behavior is added into the TemporalWorker package, inject corresponding dependencies
func NewServer(config *Config) (*Server, error) {
	temporalClient, err := config.buildTemporalClient()
	if err != nil {
		return nil, err
	}
	router := mux.NewRouter()
	router.HandleFunc("/", config.Index).Methods("GET").MatcherFunc(func(r *http.Request, rm *mux.RouteMatch) bool {
		return r.URL.Path == "/" || r.URL.Path == "/index.html"
	})
	if err != nil {
		return nil, err
	}
	server := Server{
		TemporalClient: temporalClient,
		Logger:         config.ctxLogger,
		SSLCertFile:    config.sslCertFile,
		SSLKeyFile:     config.sslKeyFile,
		StatsCloser:    config.closer,
		Router:         router,
		HealthStatus:   int32(HEALTHY),
	}
	return &server, nil
}

func (s Server) Start() error {
	s.Router.HandleFunc("/healthz", s.Healthz).Methods("GET")
	n := negroni.New(&negroni.Recovery{
		Logger:     log.New(os.Stdout, "", log.LstdFlags),
		PrintStack: false,
		StackAll:   false,
		StackSize:  1024 * 8,
	})
	n.UseHandler(s.Router)
	s.SetHealthStatus(HEALTHY)

	defer s.Logger.Close()
	defer s.TemporalClient.Close()

	// Ensure server gracefully drains connections when stopped.
	stop := make(chan os.Signal, 1)
	// Stop on SIGINTs and SIGTERMs.
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	server := &http.Server{Addr: fmt.Sprintf(":%d", s.Port), Handler: n}
	go func() {
		s.Logger.Info(fmt.Sprintf("Atlantis started - listening on port %v", s.Port))

		var err error
		if s.SSLCertFile != "" && s.SSLKeyFile != "" {
			err = server.ListenAndServeTLS(s.SSLCertFile, s.SSLKeyFile)
		} else {
			err = server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			s.Logger.Error(err.Error())
		}
	}()
	<-stop

	s.SetHealthStatus(UNHEALTHY)

	// flush stats before shutdown
	if err := s.StatsCloser.Close(); err != nil {
		s.Logger.Error(err.Error())
	}

	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second) // nolint: vet
	if err := server.Shutdown(ctx); err != nil {
		return cli.NewExitError(fmt.Sprintf("while shutting down: %s", err), 1)
	}
	return nil
}

func (s *Server) SetHealthStatus(status HealthStatus) {
	atomic.StoreInt32(&s.HealthStatus, int32(status))
}

func (s *Server) Healthz(w http.ResponseWriter, _ *http.Request) {
	var healthResponse *HealthResponse
	if atomic.LoadInt32(&s.HealthStatus) == int32(HEALTHY) {
		healthResponse = &HealthResponse{
			Status: "ok",
		}
		w.WriteHeader(http.StatusOK)
	} else {
		healthResponse = &HealthResponse{
			Status: "fail",
		}
		w.WriteHeader(http.StatusInternalServerError)
	}
	data, err := json.MarshalIndent(healthResponse, "", "  ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error creating status json response: %s", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data) // nolint: errcheck
}
