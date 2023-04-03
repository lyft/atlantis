package prrevision

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/metrics"
	internalSync "github.com/runatlantis/atlantis/server/neptune/sync"
	"github.com/runatlantis/atlantis/server/neptune/sync/crons"
	"github.com/runatlantis/atlantis/server/neptune/temporal"
	"github.com/runatlantis/atlantis/server/neptune/temporalworker/config"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/uber-go/tally/v4"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/worker"
)

const (
	// allow any in-progress PRRevision workflow executions to gracefully exit which shouldn't take longer than 10 minutes
	WorkerTimeout = 10 * time.Minute
)

type Server struct {
	Logger                   logging.Logger
	CronScheduler            *internalSync.CronScheduler
	Crons                    []*internalSync.Cron
	StatsScope               tally.Scope
	StatsCloser              io.Closer
	TemporalClient           *temporal.ClientWrapper
	GithubActivities         *activities.Github
	RevisionSetterActivities *activities.RevsionSetter
	Config                   valid.RevisionSetter
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
		"mode": "prrevision_worker",
	})

	// temporal client + worker initialization
	opts := &temporal.Options{
		StatsReporter: statsReporter,
	}
	opts = opts.WithClientInterceptors(temporal.NewMetricsInterceptor(scope))
	temporalClient, err := temporal.NewClient(config.CtxLogger, config.TemporalCfg, opts)
	if err != nil {
		return nil, errors.Wrap(err, "initializing temporal client")
	}

	cronScheduler := internalSync.NewCronScheduler(config.CtxLogger)

	githubActivities, err := activities.NewGithub(
		config.App,
		scope.SubScope("app"),
		config.DataDir,
	)
	if err != nil {
		return nil, errors.Wrap(err, "initializing github activities")
	}

	revisionSetterActivities, err := activities.NewRevisionSetter(config.RevisionSetter)
	if err != nil {
		return nil, errors.Wrap(err, "initializing revision setter activities")
	}

	server := Server{
		Logger:        config.CtxLogger,
		CronScheduler: cronScheduler,
		Crons: []*internalSync.Cron{
			{
				Executor:  crons.NewRuntimeStats(scope).Run,
				Frequency: 1 * time.Minute,
			},
		},
		StatsScope:               scope,
		StatsCloser:              statsCloser,
		TemporalClient:           temporalClient,
		GithubActivities:         githubActivities,
		RevisionSetterActivities: revisionSetterActivities,
		Config:                   config.RevisionSetter,
	}

	return &server, nil
}

func (s Server) Start() error {
	defer s.shutdown()
	var wg sync.WaitGroup

	ctx := context.Background()
	wg.Add(1)
	go s.runDefaultWorker(ctx, &wg)

	wg.Add(1)
	go s.runSlowWorker(ctx, &wg)

	// Ensure server gracefully drains connections when stopped.
	stop := make(chan os.Signal, 1)
	// Stop on SIGINTs and SIGTERMs.
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	for _, c := range s.Crons {
		s.CronScheduler.Schedule(c)
	}

	<-stop
	wg.Wait()

	return nil
}

func (s Server) shutdown() {
	s.CronScheduler.Shutdown(5 * time.Second)

	s.TemporalClient.Close()

	// flush stats before shutdown
	if err := s.StatsCloser.Close(); err != nil {
		s.Logger.Error(err.Error())
	}

	s.Logger.Close()
}

func (s Server) runDefaultWorker(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	// validated during startup, log if fails
	tqThroughPut, err := strconv.ParseFloat(s.Config.DefaultTaskQueue.ActivitiesPerSecond, 64)
	if err != nil {
		log.Fatalln(fmt.Sprintf("unable to parse task queue throughput config: %s", s.Config.DefaultTaskQueue.ActivitiesPerSecond), err)
	}

	// pass the underlying client otherwise this will panic()
	w := worker.New(s.TemporalClient.Client, workflows.PRRevisionLowThroughputTaskQueue, worker.Options{
		WorkerStopTimeout: WorkerTimeout,
		Interceptors: []interceptor.WorkerInterceptor{
			temporal.NewWorkerInterceptor(),
		},

		TaskQueueActivitiesPerSecond: tqThroughPut,
	})

	// register all activities and workflows in the default TQ
	w.RegisterWorkflow(workflows.PRRevision)
	w.RegisterActivity(s.GithubActivities)
	w.RegisterActivity(s.RevisionSetterActivities)

	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalln("unable to start pr revision default worker", err)
	}

	s.Logger.InfoContext(ctx, "Shutting down pr revision default worker, resource clean up may still be occurring in the background")
}

func (s Server) runSlowWorker(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	tqThroughPut, err := strconv.ParseFloat(s.Config.SlowTaskQueue.ActivitiesPerSecond, 64)
	if err != nil {
		log.Fatalln(fmt.Sprintf("unable to parse slow task queue throughput config: %s", s.Config.SlowTaskQueue.ActivitiesPerSecond), err)
	}

	// pass the underlying client otherwise this will panic()
	w := worker.New(s.TemporalClient.Client, workflows.PRRevisionLowThroughputTaskQueue, worker.Options{
		WorkerStopTimeout: WorkerTimeout,
		Interceptors: []interceptor.WorkerInterceptor{
			temporal.NewWorkerInterceptor(),
		},

		TaskQueueActivitiesPerSecond: tqThroughPut,
	})

	// only GH activities executes in the slow task queue
	w.RegisterActivity(s.GithubActivities)

	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalln("unable to start pr revision slow worker", err)
	}

	s.Logger.InfoContext(ctx, "Shutting down pr revision slow worker, resource clean up may still be occurring in the background")
}
