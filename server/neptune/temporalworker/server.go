package temporalworker

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/runatlantis/atlantis/server"
	"github.com/runatlantis/atlantis/server/controllers"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/uber-go/tally/v4"
	"github.com/urfave/cli"
	"github.com/urfave/negroni"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	AtlantisNamespace        = "atlantis"
	DeployTaskqueue          = "deploy"
	ProjectJobsViewRouteName = "project-jobs-detail"
)

// Config is TemporalWorker specific user config
type Config struct {
	CtxLogger        logging.Logger
	DataDir          string
	SslCertFile      string
	SslKeyFile       string
	TemporalHostPort string
	Scope            tally.Scope
	Closer           io.Closer
}

type httpServerProxy struct {
	*http.Server
	SSLCertFile string
	SSLKeyFile  string
	Logger      logging.Logger
}

func (p *httpServerProxy) ListenAndServe() {
	var err error
	if p.SSLCertFile != "" && p.SSLKeyFile != "" {
		err = p.Server.ListenAndServeTLS(p.SSLCertFile, p.SSLKeyFile)
	} else {
		err = p.Server.ListenAndServe()
	}
	if err != nil && err != http.ErrServerClosed {
		p.Logger.Error(err.Error())
	}
}

type Server struct {
	Logger           logging.Logger
	SSLCertFile      string
	SSLKeyFile       string
	Negroni          *negroni.Negroni
	LyftMode         server.Mode
	Port             int
	StatsScope       tally.Scope
	StatsCloser      io.Closer
	TemporalHostPort string
	JobsController   *controllers.JobsController
}

// TODO: as more behavior is added into the TemporalWorker package, inject corresponding dependencies
func NewServer(config *Config) (*Server, error) {
	jobsController := &controllers.JobsController{
		StatsScope: config.Scope,
	}
	// router initialization
	router := mux.NewRouter()
	router.HandleFunc("/healthz", Healthz).Methods("GET")
	router.HandleFunc("/jobs/{job-id}", jobsController.GetProjectJobs).Methods("GET").Name(ProjectJobsViewRouteName)
	router.HandleFunc("/jobs/{job-id}/ws", jobsController.GetProjectJobsWS).Methods("GET")
	n := negroni.New(&negroni.Recovery{
		Logger:     log.New(os.Stdout, "", log.LstdFlags),
		PrintStack: false,
		StackAll:   false,
		StackSize:  1024 * 8,
	})
	n.UseHandler(router)
	server := Server{
		TemporalHostPort: config.TemporalHostPort,
		Logger:           config.CtxLogger,
		SSLCertFile:      config.SslCertFile,
		SSLKeyFile:       config.SslKeyFile,
		StatsScope:       config.Scope,
		StatsCloser:      config.Closer,
		Negroni:          n,
		JobsController:   jobsController,
	}
	return &server, nil
}

func (s Server) Start() error {
	defer s.Logger.Close()

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

	s.Logger.Info(fmt.Sprintf("Atlantis started - listening on port %v", s.Port))
	httpServerProxy := httpServerProxy{
		SSLCertFile: s.SSLCertFile,
		SSLKeyFile:  s.SSLKeyFile,
		Server:      &http.Server{Addr: fmt.Sprintf(":%d", s.Port), Handler: s.Negroni},
		Logger:      s.Logger,
	}
	go httpServerProxy.ListenAndServe()
	<-stop

	// flush stats before shutdown
	if err := s.StatsCloser.Close(); err != nil {
		s.Logger.Error(err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServerProxy.Shutdown(ctx); err != nil {
		return cli.NewExitError(fmt.Sprintf("while shutting down: %s", err), 1)
	}
	return nil
}

func (s *Server) buildTemporalClient() (client.Client, error) {
	if s.TemporalHostPort != "" {
		return nil, errors.New("invalid host port")
	}
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
	// TODO: upgrade server/metrics dir to use tally v4 to be compatible with temporal
	clientOptions := client.Options{
		Namespace:         AtlantisNamespace,
		ConnectionOptions: connectionOptions,
		HostPort:          s.TemporalHostPort,
		//MetricsHandler:    temporal_tally.NewMetricsHandler(s.StatsScope),
	}
	return client.Dial(clientOptions)
}

// Healthz returns the health check response. It always returns a 200 currently.
func Healthz(w http.ResponseWriter, _ *http.Request) {
	data, err := json.MarshalIndent(&struct {
		Status string `json:"status"`
	}{
		Status: "ok",
	}, "", "  ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error creating status json response: %s", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data) // nolint: errcheck
}
