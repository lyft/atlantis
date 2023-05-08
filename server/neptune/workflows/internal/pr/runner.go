package pr

import (
	internalContext "github.com/runatlantis/atlantis/server/neptune/context"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	workflowMetrics "github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/receiver"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision"
	"go.temporal.io/sdk/workflow"
)

type Action int64

const (
	onNewRevision Action = iota
	onShutdown
)

type RunnerState int64

const (
	working RunnerState = iota
	waiting
	complete
)

type RevisionProcessor interface {
	Process(ctx workflow.Context, prRevision receiver.Revision) []activities.PolicySet
}

type Runner struct {
	RevisionSignalChannel workflow.ReceiveChannel
	RevisionReceiver      *receiver.RevisionReceiver
	ShutdownSignalChannel workflow.ReceiveChannel
	ShutdownReceiver      *receiver.ShutdownReceiver
	RevisionProcessor     RevisionProcessor
	Scope                 workflowMetrics.Scope

	// mutable state
	state                 RunnerState
	lastAttemptedRevision string
	cancel                workflow.CancelFunc
}

func newRunner(ctx workflow.Context, scope workflowMetrics.Scope, tfWorkflow revision.Workflow, internalNotifiers []revision.WorkflowNotifier) *Runner {
	revisionReceiver := receiver.NewRevisionReceiver(ctx, scope)
	shutdownReceiver := receiver.NewShutdownReceiver(ctx, scope)
	stateReceiver := revision.StateReceiver{InternalNotifiers: internalNotifiers}
	revisionProcessor := revision.Processor{
		TFWorkflow:      tfWorkflow,
		TFStateReceiver: &stateReceiver,
	}
	return &Runner{
		RevisionSignalChannel: workflow.GetSignalChannel(ctx, receiver.TerraformRevisionSignalID),
		RevisionReceiver:      &revisionReceiver,
		ShutdownSignalChannel: workflow.GetSignalChannel(ctx, receiver.ShutdownSignalID),
		ShutdownReceiver:      &shutdownReceiver,
		Scope:                 scope,
		RevisionProcessor:     &revisionProcessor,

		cancel: func() {},
	}
}

// Run handles managing the workflow's context lifecycles as new signals/poll events are received and
// change the current PRAction status
func (r *Runner) Run(ctx workflow.Context) error {
	var action Action
	var prRevision receiver.Revision

	//TODO: add approve signal, timeout, poll variation of shutdown signal
	s := workflow.NewSelector(ctx)
	s.AddReceive(r.RevisionSignalChannel, func(c workflow.ReceiveChannel, more bool) {
		prRevision = r.RevisionReceiver.Receive(c, more)
		action = onNewRevision
	})
	s.AddReceive(r.ShutdownSignalChannel, func(c workflow.ReceiveChannel, more bool) {
		r.ShutdownReceiver.Receive(c, more)
		action = onShutdown
		r.state = complete
	})

	for {
		s.Select(ctx)
		switch action {
		case onNewRevision:
			r.onNewRevision(ctx, prRevision)
		case onShutdown:
			workflow.GetLogger(ctx).Info("received shutdown signal")
			r.cancel()
			return nil
		}
	}
}

func (r *Runner) onNewRevision(ctx workflow.Context, prRevision receiver.Revision) {
	ctx = workflow.WithValue(ctx, internalContext.SHAKey, prRevision.Revision)
	workflow.GetLogger(ctx).Info("received revision signal")
	if shouldProcess := r.shouldProcessRevision(prRevision); !shouldProcess {
		return
	}
	// cancel in progress workflow (if it exists) and start up a new one
	r.cancel()
	ctx, cancel := workflow.WithCancel(ctx)
	r.cancel = cancel
	r.state = working
	r.lastAttemptedRevision = prRevision.Revision
	workflow.Go(ctx, func(c workflow.Context) {
		defer func() {
			r.state = waiting
		}()
		_ = r.RevisionProcessor.Process(c, prRevision)
		// TODO: return + filter out duplicate failed polices; check remaining failures for owner approvals
	})
}

func (r *Runner) shouldProcessRevision(prRevision receiver.Revision) bool {
	// ignore reruns when revision is still in progress
	if r.lastAttemptedRevision == prRevision.Revision && r.state != waiting {
		return false
	}
	return true
}
