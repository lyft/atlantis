package deploy

import (
	"time"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/config"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/config/logger"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision/queue"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/signals"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	"go.temporal.io/sdk/workflow"
)

const (
	TaskQueue = "deploy"

	RevisionReceiveTimeout = 60*time.Second
)

type TimedReceiver interface {
	DidTimeout() bool
	AddTimeout(ctx workflow.Context, selector workflow.Selector, timeout time.Duration)
	AddReceiveWithTimeout(ctx workflow.Context, selector workflow.Selector, timeout time.Duration)
}

type QueueWorker interface {
	Work(ctx workflow.Context)
	GetState() queue.WorkerState
}

func Workflow(ctx workflow.Context, request Request) error {
	options := workflow.ActivityOptions{
		TaskQueue:              TaskQueue,
		ScheduleToCloseTimeout: 5 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, options)

	runner := newRunner(ctx, request)

	// blocking call
	return runner.Run(ctx)
}

type Runner struct {
	QueueWorker      QueueWorker
	Selector         workflow.Selector
	RevisionReceiver TimedReceiver
}

func newRunner(ctx workflow.Context, request Request) *Runner {
	// set relevant logging context
	ctx = workflow.WithValue(ctx, config.RepositoryLogKey, request.Repository.FullName)
	ctx = workflow.WithValue(ctx, config.ProjectLogKey, request.Root.Name)
	ctx = workflow.WithValue(ctx, config.GHRequestIDLogKey, request.GHRequestID)

	// convert to internal types, we should probably move these into another struct
	repo := github.Repo{
		Name:     request.Repository.Name,
		Owner:    request.Repository.Owner,
		FullName: request.Repository.FullName,
		URL:      request.Repository.URL,
	}

	// inject dependencies

	// temporal effectively "injects" this, it just cares about the method names,
	// so we're modeling our own DI around this.
	var a *activities.Deploy

	revisionQueue := queue.NewQueue()
	revisionReceiver := signals.NewRevisionSignalReceiver(ctx, revisionQueue, )

	worker := &queue.Worker{
		Queue:      revisionQueue,
		Activities: a,
		Repo:       repo,
		RootName:   request.Root.Name,
	}

	return &Runner{
		QueueWorker:      worker,
		RevisionReceiver: revisionReceiver,
	}
}

func (r *Runner) Run(ctx workflow.Context) error {
	workerCtx, cancel := workflow.WithCancel(ctx)

	wg := workflow.NewWaitGroup(ctx)
	wg.Add(1)

	// if this panics in anyway, we'll need to ship a fix to the running workflows, else risk dropping
	// signals
	// should we have some way of persisting our queue in case of workflow termination?
	// Let's address this in a followup
	workflow.Go(ctx, func(ctx workflow.Context) {
		defer wg.Done()
		r.QueueWorker.Work(workerCtx)
	})

	selector := workflow.NewSelector(ctx)
	r.RevisionReceiver.AddReceiveWithTimeout(ctx, selector, RevisionReceiveTimeout)

	// main loop which handles external signals
	// and in turn signals the queue worker
	for {
		// blocks until a configured callback fires
		r.Selector.Select(ctx)

		if !r.RevisionReceiver.DidTimeout() {
			continue
		}

		logger.Info(ctx, "revision receiver timeout")

		// check state here since if we timed out, we're probably not susceptible to the queue
		// worker being in a waiting state right before it's about to start working on an item.
		if !r.Selector.HasPending() && r.QueueWorker.GetState() != queue.WorkingWorkerState {
			cancel()
			break
		}

		// basically keep on adding timeouts until we can either break this loop or get another signal
		r.RevisionReceiver.AddTimeout(ctx, r.Selector, RevisionReceiveTimeout)
	}
	// wait on cancellation so we can gracefully terminate, unsure if temporal handles this for us,
	// but just being safe.
	wg.Wait(ctx)

	return nil
}
