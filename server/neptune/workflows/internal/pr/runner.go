package pr

import (
	internalContext "github.com/runatlantis/atlantis/server/neptune/context"
	workflowMetrics "github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision"
	"github.com/runatlantis/atlantis/server/neptune/workflows/plugins"
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

const ShutdownSignalID = "pr-close"

type NewShutdownRequest struct{}

type RevisionProcessor interface {
	Process(ctx workflow.Context, prRevision revision.Revision)
}

type Runner struct {
	RevisionSignalChannel workflow.ReceiveChannel
	RevisionReceiver      *revision.Receiver
	ShutdownSignalChannel workflow.ReceiveChannel
	RevisionProcessor     RevisionProcessor
	Scope                 workflowMetrics.Scope

	// mutable state
	state                 RunnerState
	lastAttemptedRevision string
	cancel                workflow.CancelFunc
}

func newRunner(ctx workflow.Context, scope workflowMetrics.Scope, tfWorkflow revision.TFWorkflow, internalNotifiers []revision.WorkflowNotifier, additionalNotifiers ...plugins.TerraformWorkflowNotifier) *Runner {
	revisionReceiver := revision.NewRevisionReceiver(ctx, scope)
	stateReceiver := revision.StateReceiver{
		InternalNotifiers:   internalNotifiers,
		AdditionalNotifiers: additionalNotifiers,
	}
	revisionProcessor := revision.Processor{
		TFWorkflow:      tfWorkflow,
		TFStateReceiver: &stateReceiver,
		PolicyHandler:   &revision.FailedPolicyHandler{},
	}
	return &Runner{
		RevisionSignalChannel: workflow.GetSignalChannel(ctx, revision.TerraformRevisionSignalID),
		RevisionReceiver:      &revisionReceiver,
		ShutdownSignalChannel: workflow.GetSignalChannel(ctx, ShutdownSignalID),
		Scope:                 scope,
		RevisionProcessor:     &revisionProcessor,

		cancel: func() {},
	}
}

// Run handles managing the workflow's context lifecycles as new signals/poll events are received and
// change the current PRAction status
func (r *Runner) Run(ctx workflow.Context) error {
	var action Action
	var prRevision revision.Revision

	//TODO: add approve signal, timeout, poll variation of shutdown signal
	s := workflow.NewSelector(ctx)
	s.AddReceive(r.RevisionSignalChannel, func(c workflow.ReceiveChannel, more bool) {
		prRevision = r.RevisionReceiver.Receive(c, more)
		action = onNewRevision
	})
	s.AddReceive(r.ShutdownSignalChannel, func(c workflow.ReceiveChannel, more bool) {
		defer func() {
			action = onShutdown
			r.state = complete
		}()
		if !more {
			return
		}
		var request NewShutdownRequest
		c.Receive(ctx, &request)
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

func (r *Runner) onNewRevision(ctx workflow.Context, prRevision revision.Revision) {
	ctx = workflow.WithValue(ctx, internalContext.SHAKey, prRevision.Revision)
	workflow.GetLogger(ctx).Info("received revision signal")
	if shouldProcess := r.shouldProcessRevision(prRevision); !shouldProcess {
		//TODO: consider providing user feedback
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
		r.RevisionProcessor.Process(c, prRevision)
	})
}

func (r *Runner) shouldProcessRevision(prRevision revision.Revision) bool {
	// ignore reruns when revision is still in progress
	if r.lastAttemptedRevision == prRevision.Revision && r.state != waiting {
		return false
	}
	return true
}
