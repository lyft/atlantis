package revision

import (
	"github.com/pkg/errors"
	internalContext "github.com/runatlantis/atlantis/server/neptune/context"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	terraformActivities "github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/sideeffect"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type TFWorkflow func(ctx workflow.Context, request terraform.Request) (terraform.Response, error)

type TFStateReceiver interface {
	Receive(ctx workflow.Context, c workflow.ReceiveChannel)
	AddRoot(info RootInfo)
}

type PolicyHandler interface {
	Process(ctx workflow.Context, failedPolicies []activities.PolicySet)
}

type Processor struct {
	TFStateReceiver TFStateReceiver
	TFWorkflow      TFWorkflow
	PolicyHandler   PolicyHandler
}

// Process handles spinning off child Terraform workflows per root and
// dealing with any failed policies by reviewing set of approvals
func (p *Processor) Process(ctx workflow.Context, prRevision Revision) {
	var futures []workflow.ChildWorkflowFuture
	for _, root := range prRevision.Roots {
		future, err := p.processRoot(ctx, root, prRevision)
		if err != nil {
			// choosing to not fail workflow and let it continue to exist
			// until PR close/timeout
			workflow.GetLogger(workflow.WithValue(ctx, internalContext.ErrKey, err)).Error("generating uuid")
		} else {
			futures = append(futures, future)
		}
	}
	workflowResponses := p.awaitWorkflows(ctx, futures)
	var failedPolicies []activities.PolicySet
	for _, response := range workflowResponses {
		for _, validationResult := range response.ValidationResults {
			if validationResult.Status == activities.Fail {
				failedPolicies = append(failedPolicies, validationResult.PolicySet)
			}
		}
	}
	p.PolicyHandler.Process(ctx, failedPolicies)
}

func (p *Processor) processRoot(ctx workflow.Context, root terraformActivities.Root, prRevision Revision) (workflow.ChildWorkflowFuture, error) {
	id, err := sideeffect.GenerateUUID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "generating uuid")
	}
	p.TFStateReceiver.AddRoot(RootInfo{
		ID: id,
		Commit: github.Commit{
			Revision: prRevision.Revision,
		},
		Root: root,
		Repo: prRevision.Repo,
	})
	ctx = workflow.WithValue(ctx, internalContext.ProjectKey, root.Name)
	ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: id.String(),
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
		// allows all signals to be received even in a cancellation state
		WaitForCancellation: true,
		SearchAttributes: map[string]interface{}{
			"atlantis_repository": prRevision.Repo.GetFullName(),
			"atlantis_root":       root.Name,
			"atlantis_trigger":    root.Trigger,
			"atlantis_revision":   prRevision.Revision,
		},
	})
	request := terraform.Request{
		Repo:         prRevision.Repo,
		Root:         root,
		DeploymentID: id.String(),
		Revision:     prRevision.Revision,
		WorkflowMode: terraformActivities.PR,
	}
	future := workflow.ExecuteChildWorkflow(ctx, p.TFWorkflow, request)
	return future, nil
}

// awaitWorkflows creates a selector to listen to the completion of each root's child workflow future and any state
// change signals they send over the shared WorkflowStateChangeSignal channel; we only return when all workflows complete
func (p *Processor) awaitWorkflows(ctx workflow.Context, futures []workflow.ChildWorkflowFuture) []terraform.Response {
	selector := workflow.NewNamedSelector(ctx, "TerraformChildWorkflow")
	ch := workflow.GetSignalChannel(ctx, state.WorkflowStateChangeSignal)
	selector.AddReceive(ch, func(c workflow.ReceiveChannel, _ bool) {
		p.TFStateReceiver.Receive(ctx, c)
	})

	var results []terraform.Response
	workflowsLeft := len(futures)
	for _, future := range futures {
		selector.AddFuture(future, func(f workflow.Future) {
			defer func() {
				workflowsLeft--
			}()
			var resp terraform.Response
			if err := f.Get(ctx, &resp); err != nil {
				// we will just log terraform workflow failures and continue trying to process other futures
				workflow.GetLogger(workflow.WithValue(ctx, internalContext.ErrKey, err)).Error("executing terraform workflow")
				return
			}
			results = append(results, resp)
		})
	}

	for {
		selector.Select(ctx)
		if workflowsLeft == 0 {
			break
		}
	}
	return results
}
