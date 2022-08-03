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
	"github.com/runatlantis/atlantis/server/controllers"
	cfgParser "github.com/runatlantis/atlantis/server/core/config"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/uber-go/tally/v4"
	tallystatsd "github.com/uber-go/tally/v4/statsd"
	"github.com/urfave/cli"
	"github.com/urfave/negroni"
	"go.temporal.io/sdk/client"
	temporal_tally "go.temporal.io/sdk/contrib/tally"
	"go.temporal.io/sdk/worker"
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

const (
	AtlantisNamespace        = "atlantis"
	DeployTaskqueue          = "deploy"
	ProjectJobsViewRouteName = "project-jobs-detail"
)

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
	dataDir          string
	healthStatus     int32
	metrics          valid.Metrics
	sslCertFile      string
	sslKeyFile       string
	statsNamespace   string
	temporalHostPort string
	scope            tally.Scope
	closer           io.Closer
}

func NewConfig(atlantisURL, atlantisURLFlag, atlantisVersion, dataDir string, logLevel logging.LogLevel, repoConfig string, sslCertFile, sslKeyFile, statsNamespace, temporalHostPort string) (*Config, error) {
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
		dataDir:          dataDir,
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

type Server struct {
	Logger           logging.Logger
	SSLCertFile      string
	SSLKeyFile       string
	Router           *mux.Router
	LyftMode         server.Mode
	Port             int
	StatsScope       tally.Scope
	StatsCloser      io.Closer
	TemporalHostPort string
	HealthStatus     int32
	JobsController   *controllers.JobsController
}

// TODO: as more behavior is added into the TemporalWorker package, inject corresponding dependencies
func NewServer(config *Config) (*Server, error) {
	// TODO: fill in when statscope is upgraded to tally v4 and be compatible with temporal
	jobsController := &controllers.JobsController{}
	server := Server{
		TemporalHostPort: config.temporalHostPort,
		Logger:           config.ctxLogger,
		SSLCertFile:      config.sslCertFile,
		SSLKeyFile:       config.sslKeyFile,
		StatsScope:       config.scope,
		StatsCloser:      config.closer,
		Router:           mux.NewRouter(),
		HealthStatus:     int32(HEALTHY),
		JobsController:   jobsController,
	}
	return &server, nil
}

func (s Server) Start() error {
	defer s.Logger.Close()

	// router initialization
	s.Router.HandleFunc("/healthz", s.Healthz).Methods("GET")
	s.Router.HandleFunc("/jobs/{job-id}", s.JobsController.GetProjectJobs).Methods("GET").Name(ProjectJobsViewRouteName)
	s.Router.HandleFunc("/jobs/{job-id}/ws", s.JobsController.GetProjectJobsWS).Methods("GET")
	n := negroni.New(&negroni.Recovery{
		Logger:     log.New(os.Stdout, "", log.LstdFlags),
		PrintStack: false,
		StackAll:   false,
		StackSize:  1024 * 8,
	})
	n.UseHandler(s.Router)

	// temporal client + worker initialization
	temporalClient, err := s.buildTemporalClient()
	if err != nil {
		return err
	}
	defer temporalClient.Close()
	go func() {
		w := worker.New(temporalClient, DeployTaskqueue, worker.Options{
			// ensures that sessions are preserved on the same worker
			EnableSessionWorker: true,
		})
		if err := w.Run(worker.InterruptCh()); err != nil {
			log.Fatalln("unable to start worker", err)
		}
	}()

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

func (s *Server) buildTemporalClient() (client.Client, error) {
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
		MetricsHandler:    temporal_tally.NewMetricsHandler(s.StatsScope),
	}
	if s.TemporalHostPort != "" {
		clientOptions.HostPort = s.TemporalHostPort
	}
	return client.Dial(clientOptions)
}
