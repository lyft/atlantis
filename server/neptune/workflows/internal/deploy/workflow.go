package deploy

import (
	"time"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/config"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision/queue"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/signals"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	"go.temporal.io/sdk/workflow"
)

const (
	TaskQueue = "deploy"

	NewRevisionSignal = "new-revision"
)

// Selectable makes it easier to add multiple callbacks to a given selector
// while still allowing complete ownership of the underlying channels/futures
// to the implementation
type Selectable interface {
	AddCallback(ctx workflow.Context, selector workflow.Selector)
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
	QueueWorker *queue.Worker
	Selector    workflow.Selector
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
	newRevisionSignal := signals.NewRevisionSignal(ctx, revisionQueue, 60*time.Second)
	worker := &queue.Worker{
		Queue:      revisionQueue,
		Activities: a,
		Repo:       repo,
		RootName:   request.Root.Name,
	}
	selector := workflow.NewSelector(ctx)
	newRevisionSignal.AddCallback(ctx, selector)

	return &Runner{
		QueueWorker: worker,
		Selector:    selector,
	}
}

func (r *Runner) Run(ctx workflow.Context) error {
	ctx, cancel := workflow.WithCancel(ctx)

	wg := workflow.NewWaitGroup(ctx)
	wg.Add(1)

	workflow.Go(ctx, func(ctx workflow.Context) {
		defer wg.Done()
		r.QueueWorker.Work(ctx)
	})

	// main loop which handles external signals
	// and in turn signals the queue worker
	for {
		// blocks until a configured callback fires
		r.Selector.Select(ctx)

		// if we're waiting around doing nothing, let's just break
		if !r.Selector.HasPending() && (r.QueueWorker.GetState() == queue.WaitingWorkerState) {

			// calling cancel here is assumed to be fine since if our queue worker is waiting,
			// no deployments are in-progress
			cancel()
			break
		}
	}
	// wait on cancellation so we can gracefully terminate, unsure if temporal handles this for us,
	// but just being safe.
	wg.Wait(ctx)

	return nil
}
