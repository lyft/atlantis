package deploy

import (
	"time"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/notifier"

	"github.com/pkg/errors"
	key "github.com/runatlantis/atlantis/server/neptune/context"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision"
	revisionNotifier "github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision/notifier"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision/queue"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	workflowMetrics "github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/sideeffect"
	temporalInternal "github.com/runatlantis/atlantis/server/neptune/workflows/internal/temporal"
	"github.com/runatlantis/atlantis/server/neptune/workflows/plugins"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	TaskQueue = "deploy"

	RevisionReceiveTimeout = 60 * time.Minute

	QueueStatusNotifierHourUTC = 17

	ActiveDeployWorkflowStat  = "active"
	SuccessDeployWorkflowStat = "success"
)

type workerActivities struct {
	*activities.Github
	*activities.Deploy
}

type RunnerAction int64

const (
	OnCancel RunnerAction = iota
	OnTimeout
	OnReceive
	OnNotify
	OnUnknown
)

type container interface {
	IsEmpty() bool
}

type QueueStatusNotifier interface {
	Notify(ctx workflow.Context) error
}

type SignalReceiver interface {
	Receive(c workflow.ReceiveChannel, more bool)
}

type QueueWorker interface {
	Work(ctx workflow.Context)
	GetState() queue.WorkerState
}

type ChildWorkflows struct {
	Terraform terraform.Workflow
}

func Workflow(ctx workflow.Context, request Request, children ChildWorkflows, plugins plugins.Deploy) error {
	options := workflow.ActivityOptions{
		TaskQueue:           TaskQueue,
		StartToCloseTimeout: 5 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, options)

	runner, err := newRunner(ctx, request, children, plugins)

	if err != nil {
		return errors.Wrap(err, "initializing workflow runner")
	}

	// blocking call
	return runner.Run(ctx)
}

type Runner struct {
	Timeout                  time.Duration
	Queue                    container
	QueueWorker              QueueWorker
	RevisionReceiver         SignalReceiver
	NewRevisionSignalChannel workflow.ReceiveChannel
	Scope                    workflowMetrics.Scope
	Notifier                 QueueStatusNotifier
	NotifierPeriod           DurationGenerator
	NotifierHour             int
}

func newRunner(ctx workflow.Context, request Request, children ChildWorkflows, plugins plugins.Deploy) (*Runner, error) {
	// inject dependencies

	// temporal effectively "injects" this, it just cares about the method names,
	// so we're modeling our own DI around this.
	var a *workerActivities

	scope := workflowMetrics.NewScope(ctx)

	checkRunCache := notifier.NewGithubCheckRunCache(a)

	lockStateUpdater := queue.LockStateUpdater{
		GithubCheckRunCache: checkRunCache,
	}
	revisionQueue := queue.NewQueue(func(ctx workflow.Context, d *queue.Deploy) {
		lockStateUpdater.UpdateQueuedRevisions(ctx, d, request.Repo.FullName)
	}, scope)

	worker, err := queue.NewWorker(
		ctx,
		revisionQueue,
		a, children.Terraform, plugins.PostDeployExecutors,
		request.Repo.FullName,
		request.Root.Name,
		checkRunCache, plugins.Notifiers...)
	if err != nil {
		return nil, err
	}

	revisionReceiver := revision.NewReceiver(ctx, revisionQueue, checkRunCache, sideeffect.GenerateUUID, worker)

	return &Runner{
		Queue:                    revisionQueue,
		Timeout:                  RevisionReceiveTimeout,
		QueueWorker:              worker,
		RevisionReceiver:         revisionReceiver,
		NewRevisionSignalChannel: workflow.GetSignalChannel(ctx, revision.NewRevisionSignalID),
		Scope:                    scope,
		NotifierPeriod: func(ctx workflow.Context, hour int) time.Duration {
			return temporalInternal.UntilHour(ctx, hour, temporalInternal.NextBusinessDay)
		},
		Notifier: &revisionNotifier.Slack{
			DeployQueue: revisionQueue,
			Activities:  a,
		},
	}, nil
}

type DurationGenerator func(ctx workflow.Context, hour int) time.Duration

func (r *Runner) shutdown() {
	r.Scope.Gauge(ActiveDeployWorkflowStat).Update(0)
}

func (r *Runner) Run(ctx workflow.Context) error {
	r.Scope.Gauge(ActiveDeployWorkflowStat).Update(1)
	defer r.shutdown()

	var action RunnerAction
	workerCtx, shutdownWorker := workflow.WithCancel(ctx)

	wg := workflow.NewWaitGroup(ctx)
	wg.Add(1)

	// if this panics in anyway, we'll need to ship a fix to the running workflows, else risk dropping
	// signals
	// should we have some way of persisting our queue in case of workflow termination?
	// Let's address this in a followup
	workflow.Go(workerCtx, func(ctx workflow.Context) {
		defer wg.Done()
		r.QueueWorker.Work(ctx)
	})

	newRevisionTimerFunc := func(f workflow.Future) {
		err := f.Get(ctx, nil)

		if temporal.IsCanceledError(err) {
			action = OnCancel
			return
		}

		action = OnTimeout
	}

	s := temporalInternal.SelectorWithTimeout{
		Selector: workflow.NewSelector(ctx),
	}
	s.AddReceive(r.NewRevisionSignalChannel, func(c workflow.ReceiveChannel, more bool) {
		r.RevisionReceiver.Receive(c, more)
		action = OnReceive
	})
	cancelTimer, _ := s.AddTimeout(ctx, r.Timeout, newRevisionTimerFunc)

	notifyTimerFunc := func(f workflow.Future) {
		err := f.Get(ctx, nil)

		if err != nil {
			// this should really only happen on shutdown
			action = OnCancel
		}

		action = OnNotify
	}
	notifierPeriod := r.NotifierPeriod(ctx, QueueStatusNotifierHourUTC)
	s.AddTimeout(ctx, notifierPeriod, notifyTimerFunc)

	// main loop which handles external signals
	// and in turn signals the queue worker
OUT:
	for {
		// blocks until a configured callback fires
		s.Select(ctx)

		switch action {
		case OnCancel:
		case OnNotify:
			err := r.Notifier.Notify(ctx)
			if err != nil {
				workflow.GetLogger(ctx).Warn("Error notifying on queue status", key.ErrKey, err)
			}
			s.AddTimeout(ctx, notifierPeriod, notifyTimerFunc)
		case OnReceive:
			cancelTimer()
			cancelTimer, _ = s.AddTimeout(ctx, r.Timeout, newRevisionTimerFunc)
		case OnTimeout:
			workflow.GetLogger(ctx).Info("revision receiver timeout")

			// Since we timed out, let's determine whether we can shutdown our worker.
			// If we have no incoming revisions and the worker is just waiting, thats the first sign.
			// We also need to ensure that we're also checking whether the queue is empty since the worker can be in a waiting state
			// if the queue is locked (ie. if the workflow has just started up with prior deploy state)
			if !s.HasPending() && r.QueueWorker.GetState() != queue.WorkingWorkerState && r.Queue.IsEmpty() {
				workflow.GetLogger(ctx).Info("initiating worker shutdown")
				shutdownWorker()
				break OUT
			}

			// basically keep on adding timeouts until we can either break this loop or get another signal
			// we need to use the timeoutCtx to ensure that this gets cancelled when the receive is ready
			cancelTimer, _ = s.AddTimeout(ctx, r.Timeout, newRevisionTimerFunc)
		}
	}
	// wait on cancellation so we can gracefully terminate, unsure if temporal handles this for us,
	// but just being safe.
	wg.Wait(ctx)

	return nil
}
