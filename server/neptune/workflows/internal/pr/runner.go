package pr

import (
	"context"
	tfModel "github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/notifier"
	"time"

	metricNames "github.com/runatlantis/atlantis/server/metrics"
	internalContext "github.com/runatlantis/atlantis/server/neptune/context"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	workflowMetrics "github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision/policy"
	temporalInternal "github.com/runatlantis/atlantis/server/neptune/workflows/internal/temporal"
	"github.com/runatlantis/atlantis/server/neptune/workflows/plugins"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
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

const (
	ShutdownSignalID = "pr-close"
	TimeoutComment   = "Atlantis has deleted the state it managed for this PR due to inactivity, please rerun `atlantis plan` to rebuild the state."
)

type NewShutdownRequest struct{}

type RevisionProcessor interface {
	Process(ctx workflow.Context, prRevision revision.Revision)
}

type ShutdownChecker interface {
	ShouldShutdown(ctx workflow.Context, prRevision revision.Revision) bool
}

type ghActivities interface {
	GithubCreateComment(ctx context.Context, request activities.CreateCommentRequest) (activities.CreateCommentResponse, error)
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
	GithubActivities      ghActivities
	PRNumber              int

	// mutable state
	state                 RunnerState
	lastAttemptedRevision string
}

func newRunner(ctx workflow.Context, scope workflowMetrics.Scope, org string, tfWorkflow revision.TFWorkflow, prNum int, additionalNotifiers ...plugins.TerraformWorkflowNotifier) *Runner {
	var a *prActivities
	checkRunCache := notifier.NewGithubCheckRunCache(a)
	internalNotifiers := []revision.WorkflowNotifier{
		&notifier.CheckRunNotifier{
			CheckRunSessionCache: checkRunCache,
			Mode:                 tfModel.PR,
		},
	}
	revisionReceiver := revision.NewRevisionReceiver(ctx, scope)
	stateReceiver := revision.StateReceiver{
		InternalNotifiers:   internalNotifiers,
		AdditionalNotifiers: additionalNotifiers,
	}
	var ga *activities.Github
	dismisser := policy.StaleReviewDismisser{
		GithubActivities: ga,
		PRNumber:         prNum,
	}
	revisionProcessor := revision.Processor{
		TFWorkflow:      tfWorkflow,
		TFStateReceiver: &stateReceiver,
		PolicyHandler: &policy.FailedPolicyHandler{
			ReviewSignalChannel: workflow.GetSignalChannel(ctx, revision.ReviewSignalID),
			GithubActivities:    ga,
			PRNumber:            prNum,
			Dismisser:           &dismisser,
			PolicyFilter:        &policy.Filter{},
			Org:                 org,
			Scope:               scope,
			Notifier:            &stateReceiver,
		},
		GithubCheckRunCache: checkRunCache,
		Scope:               scope,
	}
	shutdownChecker := ShutdownStateChecker{
		GithubActivities: ga,
		PRNumber:         prNum,
	}
	return &Runner{
		RevisionSignalChannel: workflow.GetSignalChannel(ctx, revision.TerraformRevisionSignalID),
		RevisionReceiver:      &revisionReceiver,
		ShutdownSignalChannel: workflow.GetSignalChannel(ctx, ShutdownSignalID),
		Scope:                 scope,
		RevisionProcessor:     &revisionProcessor,
		ShutdownChecker:       &shutdownChecker,
		GithubActivities:      ga,
		PRNumber:              prNum,

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
		r.Scope.Counter(metricNames.ShutdownTimeout).Inc(1)
	}
	inactivityTimeoutCancel, _ := s.AddTimeout(ctx, r.InactivityTimeout, onInactivityTimeout)

	s.AddReceive(r.RevisionSignalChannel, func(c workflow.ReceiveChannel, more bool) {
		prRevision = r.RevisionReceiver.Receive(c, more)
		action = onNewRevision
		tags := map[string]string{
			metricNames.SignalNameTag: revision.TerraformRevisionSignalID,
			metricNames.RevisionTag:   prRevision.Revision,
		}
		r.Scope.SubScopeWithTags(tags).
			Counter(metricNames.SignalReceive).
			Inc(1)
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
		r.Scope.SubScopeWithTags(map[string]string{metricNames.SignalNameTag: ShutdownSignalID}).
			Counter(metricNames.SignalReceive).
			Inc(1)
	})

	onShutdownPollTick := func(f workflow.Future) {
		action = onShutdownPoll
		r.Scope.SubScopeWithTags(map[string]string{metricNames.PollNameTag: ShutdownSignalID}).
			Counter(metricNames.PollTick).
			Inc(1)
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
		case onTimeout:
			workflow.GetLogger(ctx).Info("workflow timed out, shutting down")
			r.notifyOnTimeout(ctx, prRevision)
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

func (r *Runner) notifyOnTimeout(ctx workflow.Context, prRevision revision.Revision) {
	ctx = workflow.WithRetryPolicy(ctx, temporal.RetryPolicy{
		MaximumAttempts: 3,
	})
	createCommentRequest := activities.CreateCommentRequest{
		Repo:        prRevision.Repo,
		PRNumber:    r.PRNumber,
		CommentBody: TimeoutComment,
	}
	err := workflow.ExecuteActivity(ctx, r.GithubActivities.GithubCreateComment, createCommentRequest).Get(ctx, nil)
	if err != nil {
		// we won't fail workflow if we can't create this comment
		workflow.GetLogger(ctx).Error("creating github comment")
	}
}
