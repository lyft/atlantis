package pr

import (
	internalContext "github.com/runatlantis/atlantis/server/neptune/context"
	workflowMetrics "github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision/policy"
	temporalInternal "github.com/runatlantis/atlantis/server/neptune/workflows/internal/temporal"
	"github.com/runatlantis/atlantis/server/neptune/workflows/plugins"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"time"
)

type Action int64

const (
	onNewRevision Action = iota
	onShutdownPoll
	onShutdown
	onCancel
	onTimeout
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

type ShutdownChecker interface {
	ShouldShutdown(ctx workflow.Context, prRevision revision.Revision) bool
}

type Runner struct {
	RevisionSignalChannel workflow.ReceiveChannel
	RevisionReceiver      *revision.Receiver
	ShutdownSignalChannel workflow.ReceiveChannel
	RevisionProcessor     RevisionProcessor
	ShutdownChecker       ShutdownChecker
	Scope                 workflowMetrics.Scope
	InactivityTimeout     time.Duration
	ShutdownPollTick      time.Duration

	// mutable state
	state                 RunnerState
	lastAttemptedRevision string
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
		PolicyHandler: &policy.FailedPolicyHandler{
			ApprovalSignalChannel: workflow.GetSignalChannel(ctx, revision.ApprovalSignalID),
			// TODO: populate other fields, will do so once another pr (640) is merged
		},
	}
	shutdownChecker := ShutdownStateChecker{}
	return &Runner{
		RevisionSignalChannel: workflow.GetSignalChannel(ctx, revision.TerraformRevisionSignalID),
		RevisionReceiver:      &revisionReceiver,
		ShutdownSignalChannel: workflow.GetSignalChannel(ctx, ShutdownSignalID),
		Scope:                 scope,
		RevisionProcessor:     &revisionProcessor,
		ShutdownChecker:       &shutdownChecker,

		// TODO: make these configurations
		InactivityTimeout: time.Hour * 24 * 7,
		ShutdownPollTick:  time.Hour * 24,
	}
}

// Run handles managing the workflow's context lifecycles as new signals/poll events are received and
// change the current PRAction status
func (r *Runner) Run(ctx workflow.Context) error {
	var action Action
	var prRevision revision.Revision

	s := temporalInternal.SelectorWithTimeout{
		Selector: workflow.NewSelector(ctx),
	}
	onInactivityTimeout := func(f workflow.Future) {
		err := f.Get(ctx, nil)
		if temporal.IsCanceledError(err) {
			action = onCancel
			return
		}
		action = onTimeout
	}
	inactivityTimeoutCancel, _ := s.AddTimeout(ctx, r.InactivityTimeout, onInactivityTimeout)

	s.AddReceive(r.RevisionSignalChannel, func(c workflow.ReceiveChannel, more bool) {
		prRevision = r.RevisionReceiver.Receive(c, more)
		action = onNewRevision
	})
	s.AddReceive(r.ShutdownSignalChannel, func(c workflow.ReceiveChannel, more bool) {
		defer func() {
			action = onShutdown
		}()
		if !more {
			return
		}
		var request NewShutdownRequest
		c.Receive(ctx, &request)
	})

	onShutdownPollTick := func(f workflow.Future) {
		action = onShutdownPoll
	}
	shutdownPollCancel, _ := s.AddTimeout(ctx, r.ShutdownPollTick, onShutdownPollTick)

	_, revisionCancel := workflow.WithCancel(ctx)
	for {
		s.Select(ctx)
		switch action {
		case onNewRevision:
			revisionCancel = r.onNewRevision(ctx, revisionCancel, prRevision)
			inactivityTimeoutCancel()
			inactivityTimeoutCancel, _ = s.AddTimeout(ctx, r.InactivityTimeout, onInactivityTimeout)
			continue
		case onCancel:
			continue
		case onTimeout: // TODO: send message to PR stating atlantis deleted state due to inactivity and to rerun to trigger atlantis workflow
			workflow.GetLogger(ctx).Info("workflow timed out, shutting down")
			revisionCancel()
			return nil
		case onShutdown:
			workflow.GetLogger(ctx).Info("received shutdown signal")
		case onShutdownPoll:
			workflow.GetLogger(ctx).Info("shutdown check poll tick")
		}
		// shutdown check
		shutdownPollCancel()
		if shutdown := r.ShutdownChecker.ShouldShutdown(ctx, prRevision); !shutdown {
			shutdownPollCancel, _ = s.AddTimeout(ctx, r.ShutdownPollTick, onShutdownPollTick)
			continue
		}
		revisionCancel()
		return nil
	}
}

func (r *Runner) onNewRevision(ctx workflow.Context, cancel workflow.CancelFunc, prRevision revision.Revision) workflow.CancelFunc {
	ctx = workflow.WithValue(ctx, internalContext.SHAKey, prRevision.Revision)
	workflow.GetLogger(ctx).Info("received revision signal")
	if shouldProcess := r.shouldProcessRevision(prRevision); !shouldProcess {
		//TODO: consider providing user feedback
		return cancel
	}
	// cancel in progress workflow (if it exists) and start up a new one
	cancel()
	ctx, cancel = workflow.WithCancel(ctx)
	r.state = working
	r.lastAttemptedRevision = prRevision.Revision
	workflow.Go(ctx, func(c workflow.Context) {
		defer func() {
			r.state = waiting
		}()
		r.RevisionProcessor.Process(c, prRevision)
	})
	return cancel
}

func (r *Runner) shouldProcessRevision(prRevision revision.Revision) bool {
	// ignore reruns when revision is still in progress
	if r.lastAttemptedRevision == prRevision.Revision && r.state != waiting {
		return false
	}
	return true
}
