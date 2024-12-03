package terraform

import (
	"github.com/pkg/errors"
	terraformActivities "github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/lock"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"github.com/runatlantis/atlantis/server/neptune/workflows/plugins"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const DivergedMetric = "diverged"

type PlanRejectionError struct {
	msg string
}

func NewPlanRejectionError(msg string) *PlanRejectionError {
	return &PlanRejectionError{
		msg: msg,
	}
}

func (e PlanRejectionError) Error() string {
	return e.msg
}

type Workflow func(ctx workflow.Context, request terraform.Request) (terraform.Response, error)

type stateReceiver interface {
	Receive(ctx workflow.Context, c workflow.ReceiveChannel, deploymentInfo DeploymentInfo)
}

type deployQueue interface {
	GetOrderedMergedItems() []DeploymentInfo
	GetQueuedRevisionsSummary() string
	GetLockState() lock.LockState
}

func NewWorkflowRunner(queue deployQueue, w Workflow, githubCheckRunCache CheckRunClient, internalNotifiers []WorkflowNotifier, additionalNotifiers ...plugins.TerraformWorkflowNotifier) *WorkflowRunner {
	return &WorkflowRunner{
		Workflow: w,
		StateReceiver: &StateReceiver{
			Queue:               queue,
			CheckRunCache:       githubCheckRunCache,
			InternalNotifiers:   internalNotifiers,
			AdditionalNotifiers: additionalNotifiers,
		},
	}
}

type WorkflowRunner struct {
	StateReceiver stateReceiver
	Workflow      Workflow
}

func (r *WorkflowRunner) Run(ctx workflow.Context, deploymentInfo DeploymentInfo, planApproval terraformActivities.PlanApproval, scope metrics.Scope) error {
	id := deploymentInfo.ID
	ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: id.String(),
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},

		// allows all signals to be received even in a cancellation state
		WaitForCancellation: true,
		SearchAttributes: map[string]interface{}{
			"atlantis_repository": deploymentInfo.Repo.GetFullName(),
			"atlantis_root":       deploymentInfo.Root.Name,
			"atlantis_trigger":    deploymentInfo.Root.TriggerInfo.Type,
			"atlantis_revision":   deploymentInfo.Commit.Revision,
		},
	})

	request := terraform.Request{
		Repo:         deploymentInfo.Repo,
		Root:         deploymentInfo.Root.WithPlanApprovalOverride(planApproval),
		DeploymentID: id.String(),
		Revision:     deploymentInfo.Commit.Revision,
		WorkflowMode: terraformActivities.Deploy,
	}

	future := workflow.ExecuteChildWorkflow(ctx, r.Workflow, request)
	return r.awaitWorkflow(ctx, future, deploymentInfo)
}

func (r *WorkflowRunner) awaitWorkflow(ctx workflow.Context, future workflow.ChildWorkflowFuture, deploymentInfo DeploymentInfo) error {
	selector := workflow.NewNamedSelector(ctx, "TerraformChildWorkflow")

	// our child workflow will signal us when there is a state change which we will handle accordingly.
	// if for some reason the workflow is orphaned or we are retrying it independently, we have no way
	// to really update the state since we won't be listening for that signal anymore
	// we could have moved this to the main selector in the worker however we wouldn't always have this deployment info
	// which is necessary for knowing which check run id to update.
	// TODO: figure out how to solve this
	ch := workflow.GetSignalChannel(ctx, state.WorkflowStateChangeSignal)
	selector.AddReceive(ch, func(c workflow.ReceiveChannel, _ bool) {
		r.StateReceiver.Receive(ctx, c, deploymentInfo)
	})
	var workflowComplete bool
	var err error
	selector.AddFuture(future, func(f workflow.Future) {
		workflowComplete = true
		err = f.Get(ctx, nil)
	})

	for {
		selector.Select(ctx)

		if workflowComplete {
			break
		}
	}

	// if we have an app error we should attempt to unwrap it's details into our own
	// application error and act accordingly
	var appErr *temporal.ApplicationError
	if errors.As(err, &appErr) {
		unwrapped := errors.Unwrap(appErr)

		var msg string
		if unwrapped != nil {
			msg = unwrapped.Error()
		} else {
			msg = "plan has been rejected"
		}
		if appErr.Type() == terraform.PlanRejectedErrorType {
			return NewPlanRejectionError(msg)
		}
	}

	return errors.Wrap(err, "executing terraform workflow")
}
