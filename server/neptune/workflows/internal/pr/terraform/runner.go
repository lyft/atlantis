package terraform

import (
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	terraformActivities "github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type Workflow func(ctx workflow.Context, request terraform.Request) (terraform.Response, error)

type stateReceiver interface {
	Receive(ctx workflow.Context, c workflow.ReceiveChannel, prInfo PRRootInfo)
}

func NewWorkflowRunner(w Workflow, internalNotifiers []WorkflowNotifier) *WorkflowRunner {
	return &WorkflowRunner{
		Workflow: w,
		StateReceiver: &StateReceiver{
			InternalNotifiers: internalNotifiers,
		},
	}
}

type WorkflowRunner struct {
	StateReceiver stateReceiver
	Workflow      Workflow
}

func (r *WorkflowRunner) Run(ctx workflow.Context, prRootInfo PRRootInfo) (map[string]activities.PolicySet, error) {
	id := prRootInfo.ID
	ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: id.String(),
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},

		// allows all signals to be received even in a cancellation state
		WaitForCancellation: true,
		SearchAttributes: map[string]interface{}{
			"atlantis_repository": prRootInfo.Repo.GetFullName(),
			"atlantis_root":       prRootInfo.Root.Name,
			"atlantis_trigger":    prRootInfo.Root.Trigger,
			"atlantis_revision":   prRootInfo.Commit.Revision,
		},
	})

	request := terraform.Request{
		Repo:         prRootInfo.Repo,
		Root:         prRootInfo.Root,
		DeploymentID: id.String(),
		Revision:     prRootInfo.Commit.Revision,
		WorkflowMode: terraformActivities.PR,
	}

	future := workflow.ExecuteChildWorkflow(ctx, r.Workflow, request)
	return r.awaitWorkflow(ctx, future, prRootInfo)
}

func (r *WorkflowRunner) awaitWorkflow(ctx workflow.Context, future workflow.ChildWorkflowFuture, prInfo PRRootInfo) (map[string]activities.PolicySet, error) {
	selector := workflow.NewNamedSelector(ctx, "TerraformChildWorkflow")
	ch := workflow.GetSignalChannel(ctx, state.WorkflowStateChangeSignal)
	selector.AddReceive(ch, func(c workflow.ReceiveChannel, _ bool) {
		r.StateReceiver.Receive(ctx, c, prInfo)
	})
	var workflowComplete bool
	var err error
	failedPolicies := make(map[string]activities.PolicySet)
	selector.AddFuture(future, func(f workflow.Future) {
		workflowComplete = true
		var resp terraform.Response
		err = f.Get(ctx, &resp)
		for _, result := range resp.ValidationResults {
			if result.Status == activities.Fail {
				failedPolicies[result.PolicySet.Name] = result.PolicySet
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
