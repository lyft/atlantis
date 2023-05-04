package pr

import (
	"github.com/pkg/errors"
	internalContext "github.com/runatlantis/atlantis/server/neptune/context"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	terraformActivities "github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	workflowMetrics "github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/receiver"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/sideeffect"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type TFWorkflow func(ctx workflow.Context, request terraform.Request) (terraform.Response, error)

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

type Runner struct {
	RevisionSignalChannel workflow.ReceiveChannel
	RevisionReceiver      *receiver.RevisionReceiver
	ShutdownSignalChannel workflow.ReceiveChannel
	ShutdownReceiver      *receiver.ShutdownReceiver
	Scope                 workflowMetrics.Scope
	TFWorkflow            TFWorkflow
	TFStateReceiver       StateReceiver

	// mutable state
	state                 RunnerState
	lastAttemptedRevision string
}

func newRunner(ctx workflow.Context, scope workflowMetrics.Scope, tfWorkflow TFWorkflow, internalNotifiers []WorkflowNotifier) *Runner {
	revisionReceiver := receiver.NewRevisionReceiver(ctx, scope)
	shutdownReceiver := receiver.NewShutdownReceiver(ctx, scope)
	return &Runner{
		RevisionSignalChannel: workflow.GetSignalChannel(ctx, receiver.TerraformRevisionSignalID),
		RevisionReceiver:      &revisionReceiver,
		ShutdownSignalChannel: workflow.GetSignalChannel(ctx, receiver.ShutdownSignalID),
		ShutdownReceiver:      &shutdownReceiver,
		Scope:                 scope,
		TFWorkflow:            tfWorkflow,
		TFStateReceiver:       StateReceiver{InternalNotifiers: internalNotifiers},
	}
}

// Run handles managing the workflow's context lifecycles as new signals/poll events are received and
// change the current PRAction status
func (r *Runner) Run(ctx workflow.Context) error {
	var action Action
	var prRevision receiver.Revision

	//TODO: add approve signal, timeouts, poll variation of shutdown signal
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

	_, cancel := workflow.WithCancel(ctx)
	for {
		s.Select(ctx)
		switch action {
		case onNewRevision:
			revisionCtx := workflow.WithValue(ctx, internalContext.SHAKey, prRevision.Revision)
			workflow.GetLogger(revisionCtx).Info("received revision")
			if process := r.shouldProcessRevision(prRevision); !process {
				continue
			}
			// cancel in progress workflow (if it exists) and start up a new one
			cancel()
			revisionCtx, cancel = workflow.WithCancel(revisionCtx)
			r.state = working
			r.lastAttemptedRevision = prRevision.Revision
			workflow.Go(revisionCtx, func(c workflow.Context) {
				r.processRevision(c, prRevision)
			})
		case onShutdown:
			//todo: maybe optimize by cancelling in progress child workflows
			workflow.GetLogger(ctx).Info("received shutdown request")
			return nil
		}
	}
}

func (r *Runner) shouldProcessRevision(prRevision receiver.Revision) bool {
	// ignore reruns when revision is still in progress
	if r.lastAttemptedRevision == prRevision.Revision && r.state != waiting {
		return false
	}
	return true
}

// processRevision handles spinning off child Terraform workflows per root and
// dealing with any failed policies by reviewing set of approvals
func (r *Runner) processRevision(ctx workflow.Context, prRevision receiver.Revision) {
	defer func() {
		r.state = waiting
	}()
	failedPolicies := make(map[string]activities.PolicySet)
	var futures []workflow.ChildWorkflowFuture
	var rootInfos []RootInfo
	for _, root := range prRevision.Roots {
		ctx = workflow.WithValue(ctx, internalContext.ProjectKey, root.Name)
		future, rootInfo, err := r.processRoot(ctx, root, prRevision)
		if err != nil {
			continue
		}
		futures = append(futures, future)
		rootInfos = append(rootInfos, rootInfo)
	}
	for i, future := range futures {
		failedRootPolicies, err := r.awaitWorkflow(ctx, future, rootInfos[i])
		if err != nil {
			continue
		}
		// consolidate failures across all roots
		// policy sets are identical so multiple roots can fail the same policy without issue
		for _, failedPolicy := range failedRootPolicies {
			failedPolicies[failedPolicy.Name] = failedPolicy
		}
	}
	// TODO: check for policy failures
}

func (r *Runner) processRoot(ctx workflow.Context, root terraformActivities.Root, prRevision receiver.Revision) (workflow.ChildWorkflowFuture, RootInfo, error) {
	id, err := sideeffect.GenerateUUID(ctx)
	if err != nil {
		workflow.GetLogger(workflow.WithValue(ctx, internalContext.ErrKey, err)).Error("generating uuid")
		// choosing to not fail workflow and let it continue to exist
		// until PR close/timeout
		return nil, RootInfo{}, err
	}
	rootInfo := RootInfo{
		ID: id,
		Commit: github.Commit{
			Revision: prRevision.Revision,
		},
		Root: root,
		Repo: prRevision.Repo,
	}
	ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: rootInfo.ID.String(),
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},

		// allows all signals to be received even in a cancellation state
		WaitForCancellation: true,
		SearchAttributes: map[string]interface{}{
			"atlantis_repository": rootInfo.Repo.GetFullName(),
			"atlantis_root":       rootInfo.Root.Name,
			"atlantis_trigger":    rootInfo.Root.Trigger,
			"atlantis_revision":   rootInfo.Commit.Revision,
		},
	})

	request := terraform.Request{
		Repo:         rootInfo.Repo,
		Root:         rootInfo.Root,
		DeploymentID: id.String(),
		Revision:     rootInfo.Commit.Revision,
		WorkflowMode: terraformActivities.PR,
	}
	future := workflow.ExecuteChildWorkflow(ctx, r.TFWorkflow, request)
	return future, rootInfo, nil
}

func (r *Runner) awaitWorkflow(ctx workflow.Context, future workflow.ChildWorkflowFuture, rootInfo RootInfo) ([]activities.PolicySet, error) {
	selector := workflow.NewNamedSelector(ctx, "TerraformChildWorkflow")
	ch := workflow.GetSignalChannel(ctx, state.WorkflowStateChangeSignal)
	selector.AddReceive(ch, func(c workflow.ReceiveChannel, _ bool) {
		r.TFStateReceiver.Receive(ctx, c, rootInfo)
	})
	var workflowComplete bool
	var err error
	var failedPolicies []activities.PolicySet
	selector.AddFuture(future, func(f workflow.Future) {
		workflowComplete = true
		var resp terraform.Response
		err = f.Get(ctx, &resp)
		for _, result := range resp.ValidationResults {
			if result.Status == activities.Fail {
				failedPolicies = append(failedPolicies, result.PolicySet)
			}
		}
	})
	for {
		selector.Select(ctx)
		if workflowComplete {
			break
		}
	}
	return failedPolicies, errors.Wrap(err, "executing terraform workflow")
}
