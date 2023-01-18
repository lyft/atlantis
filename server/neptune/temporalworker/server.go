package temporalworker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	awsSns "github.com/aws/aws-sdk-go/service/sns"
	assetfs "github.com/elazarl/go-bindata-assetfs"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/lyft/aws"
	"github.com/runatlantis/atlantis/server/metrics"
	neptune_http "github.com/runatlantis/atlantis/server/neptune/http"
	internalSync "github.com/runatlantis/atlantis/server/neptune/sync"
	"github.com/runatlantis/atlantis/server/neptune/sync/crons"
	"github.com/runatlantis/atlantis/server/neptune/temporal"
	"github.com/runatlantis/atlantis/server/neptune/temporalworker/config"
	"github.com/runatlantis/atlantis/server/neptune/temporalworker/controllers"
	"github.com/runatlantis/atlantis/server/neptune/temporalworker/job"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/aws/sns"
	"github.com/runatlantis/atlantis/server/static"
	"github.com/uber-go/tally/v4"
	"github.com/urfave/negroni"
	"go.temporal.io/sdk/worker"
)

const (
	ProjectJobsViewRouteName = "project-jobs-detail"

	// Equal to default terraform timeout
	TemporalWorkerTimeout = time.Hour

	// 5 minutes to allow cleaning up the job store
	StreamHandlerTimeout = 5 * time.Minute
)

type Server struct {
	Logger            logging.Logger
	HTTPServerProxy   *neptune_http.ServerProxy
	CronScheduler     *internalSync.CronScheduler
	Crons             []*internalSync.Cron
	Port              int
	StatsScope        tally.Scope
	StatsCloser       io.Closer
	TemporalClient    *temporal.ClientWrapper
	JobStreamHandler  *job.StreamHandler
	JobStreamCloserFn job.StreamCloserFn

	DeployActivities    *activities.Deploy
	TerraformActivities *activities.Terraform
	GithubActivities    *activities.Github
	TerraformTaskQueue  string
}

func NewServer(config *config.Config) (*Server, error) {
	statsReporter, err := metrics.NewReporter(config.Metrics, config.CtxLogger)

	if err != nil {
		return nil, err
	}

	scope, statsCloser := metrics.NewScopeWithReporter(config.Metrics, config.CtxLogger, config.StatsNamespace, statsReporter)
	if err != nil {
		return nil, err
	}

	scope = scope.Tagged(map[string]string{
		"mode": "worker",
	})

	// Build dependencies required for output handler and jobs controller
	jobStore, err := job.NewStorageBackendStore(config.JobConfig, scope.SubScope("job.store"), config.CtxLogger)
	if err != nil {
		return nil, errors.Wrapf(err, "initializing job store")
	}
	receiverRegistry := job.NewReceiverRegistry()

	// terraform job output handler
	jobStreamHandler, streamCloserFn := job.NewStreamHandler(jobStore, receiverRegistry, config.TerraformCfg.LogFilters, config.CtxLogger)
	jobsController := controllers.NewJobsController(jobStore, receiverRegistry, config.ServerCfg, scope, config.CtxLogger)

	// temporal client + worker initialization
	opts := &temporal.Options{
		StatsReporter: statsReporter,
	}
	opts = opts.WithClientInterceptors(temporal.NewMetricsInterceptor(scope))
	temporalClient, err := temporal.NewClient(config.CtxLogger, config.TemporalCfg, opts)
	if err != nil {
		return nil, errors.Wrap(err, "initializing temporal client")
	}

	// router initialization
	router := mux.NewRouter()
	router.HandleFunc("/healthz", Healthz).Methods("GET")
	router.PathPrefix("/static/").Handler(http.FileServer(&assetfs.AssetFS{Asset: static.Asset, AssetDir: static.AssetDir, AssetInfo: static.AssetInfo}))
	router.HandleFunc("/jobs/{job-id}", jobsController.GetProjectJobs).Methods("GET").Name(ProjectJobsViewRouteName)
	router.HandleFunc("/jobs/{job-id}/ws", jobsController.GetProjectJobsWS).Methods("GET")
	n := negroni.New(&negroni.Recovery{
		Logger:     log.New(os.Stdout, "", log.LstdFlags),
		PrintStack: false,
		StackAll:   false,
		StackSize:  1024 * 8,
	})
	n.UseHandler(router)
	httpServerProxy := &neptune_http.ServerProxy{
		SSLCertFile: config.AuthCfg.SslCertFile,
		SSLKeyFile:  config.AuthCfg.SslKeyFile,
		Server:      &http.Server{Addr: fmt.Sprintf(":%d", config.ServerCfg.Port), Handler: n, ReadHeaderTimeout: time.Second * 10},
		Logger:      config.CtxLogger,
	}

	//TODO: move this within the activity construction
	// we only initialize the AWS session if we have a topic, otherwise we just drop the message,
	// for now this is how we get around local testing without aws resources.
	var snsWriter io.Writer
	if config.LyftAuditJobsSnsTopicArn != "" {
		session, err := aws.NewSession()
		if err != nil {
			return nil, errors.Wrap(err, "initializing new aws session")
		}
		snsWriter = &sns.Writer{
			Client:   awsSns.New(session),
			TopicArn: &config.LyftAuditJobsSnsTopicArn,
		}
	} else {
		snsWriter = io.Discard
	}
	deployActivities, err := activities.NewDeploy(config.DeploymentConfig, snsWriter)
	if err != nil {
		return nil, errors.Wrap(err, "initializing deploy activities")
	}

	terraformActivities, err := activities.NewTerraform(
		config.TerraformCfg,
		config.App,
		config.DataDir,
		config.ServerCfg.URL,
		config.TemporalCfg.TerraformTaskQueue,
		jobStreamHandler,
	)
	if err != nil {
		return nil, errors.Wrap(err, "initializing terraform activities")
	}

	githubActivities, err := activities.NewGithub(
		config.App,
		scope.SubScope("app"),
		config.DataDir,
	)
	if err != nil {
		return nil, errors.Wrap(err, "initializing github activities")
	}

	cronScheduler := internalSync.NewCronScheduler(config.CtxLogger)

	server := Server{
		Logger:        config.CtxLogger,
		CronScheduler: cronScheduler,
		Crons: []*internalSync.Cron{
			{
				Executor:  crons.NewRuntimeStats(scope).Run,
				Frequency: 1 * time.Minute,
			},
		},
		HTTPServerProxy:     httpServerProxy,
		Port:                config.ServerCfg.Port,
		StatsScope:          scope,
		StatsCloser:         statsCloser,
		TemporalClient:      temporalClient,
		JobStreamHandler:    jobStreamHandler,
		JobStreamCloserFn:   streamCloserFn,
		DeployActivities:    deployActivities,
		TerraformActivities: terraformActivities,
		GithubActivities:    githubActivities,
		TerraformTaskQueue:  config.TemporalCfg.TerraformTaskQueue,
	}
	return &server, nil
}

func (s Server) Start() error {
	defer s.shutdown()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		deployWorker := s.buildDeployWorker()
		if err := deployWorker.Run(worker.InterruptCh()); err != nil {
			log.Fatalln("unable to start deploy worker", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		// Close job stream when temporalworker exits to allow gracefully shutting down the stream handler
		defer s.JobStreamCloserFn()

		terraformWorker := s.buildTerraformWorker()
		if err := terraformWorker.Run(worker.InterruptCh()); err != nil {
			log.Fatalln("unable to start terraform worker", err)
		}
	}()

	// Ensure server gracefully drains connections when stopped.
	stop := make(chan os.Signal, 1)
	// Stop on SIGINTs and SIGTERMs.
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	s.Logger.Info(fmt.Sprintf("Atlantis started - listening on port %v", s.Port))

	go func() {
		err := s.HTTPServerProxy.ListenAndServe()

		if err != nil && err != http.ErrServerClosed {
			s.Logger.Error(err.Error())
		}
	}()

	// Start job output handler listener
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Exits when the job output chan is closed which happens only after the temporal worker
		// has gracefully shut down
		s.JobStreamHandler.Handle()
	}()

	for _, c := range s.Crons {
		s.CronScheduler.Schedule(c)
	}

	<-stop
	wg.Wait()

	return nil
}

func (s Server) shutdown() {
	// On cleanup, stream handler closes all active receivers and persists in memory jobs to storage
	ctx, cancel := context.WithTimeout(context.Background(), StreamHandlerTimeout)
	defer cancel()
	if err := s.JobStreamHandler.CleanUp(ctx); err != nil {
		s.Logger.Error(err.Error())
	}

	// flush stats before shutdown
	if err := s.StatsCloser.Close(); err != nil {
		s.Logger.Error(err.Error())
	}

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.HTTPServerProxy.Shutdown(ctx); err != nil {
		s.Logger.Error(err.Error())
	}

	s.CronScheduler.Shutdown(5 * time.Second)

	s.Logger.Close()
	s.TemporalClient.Close()
}

// // allows us to know which worker is polling which task queue without having to
// // dig through the UI
// // additionally, this also allows us to share our temporal client across workers
// func generateWorkerID(taskQueue string) string {
// 	hostname, err := os.Hostname()
// 	if err != nil {
// 		hostname = "Unknown"
// 	}

// 	return fmt.Sprintf("%d@%s@%s", os.Getpid(), hostname, taskQueue)
// }

func (s Server) buildDeployWorker() worker.Worker {
	// pass the underlying client otherwise this will panic()
	deployWorker := worker.New(s.TemporalClient.Client, workflows.DeployTaskQueue, worker.Options{
		WorkerStopTimeout: TemporalWorkerTimeout,
		// Identity:          generateWorkerID(workflows.DeployTaskQueue),
	})
	deployWorker.RegisterActivity(s.DeployActivities)
	deployWorker.RegisterActivity(s.GithubActivities)
	deployWorker.RegisterActivity(s.TerraformActivities)
	deployWorker.RegisterWorkflow(workflows.Deploy)
	deployWorker.RegisterWorkflow(workflows.Terraform)
	return deployWorker
}

func (s Server) buildTerraformWorker() worker.Worker {
	// pass the underlying client otherwise this will panic()
	terraformWorker := worker.New(s.TemporalClient.Client, s.TerraformTaskQueue, worker.Options{
		WorkerStopTimeout: TemporalWorkerTimeout,
		// Identity:          generateWorkerID(s.TerraformTaskQueue),
	})
	terraformWorker.RegisterActivity(s.TerraformActivities)
	terraformWorker.RegisterActivity(s.GithubActivities)
	terraformWorker.RegisterWorkflow(workflows.Terraform)
	return terraformWorker
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
