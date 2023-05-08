package revision

import (
	internalContext "github.com/runatlantis/atlantis/server/neptune/context"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	terraformActivities "github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/receiver"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/sideeffect"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type Workflow func(ctx workflow.Context, request terraform.Request) (terraform.Response, error)

type Receiver interface {
	Receive(ctx workflow.Context, c workflow.ReceiveChannel, rootInfo RootInfo)
}

type Processor struct {
	TFStateReceiver Receiver
	TFWorkflow      Workflow
}

// Process handles spinning off child Terraform workflows per root and
// dealing with any failed policies by reviewing set of approvals
func (p *Processor) Process(ctx workflow.Context, prRevision receiver.Revision) []activities.PolicySet {
	var futures []workflow.ChildWorkflowFuture
	var rootInfos []RootInfo
	for _, root := range prRevision.Roots {
		ctx = workflow.WithValue(ctx, internalContext.ProjectKey, root.Name)
		id, err := sideeffect.GenerateUUID(ctx)
		if err != nil {
			// choosing to not fail workflow and let it continue to exist
			// until PR close/timeout
			workflow.GetLogger(workflow.WithValue(ctx, internalContext.ErrKey, err)).Error("generating uuid")
			continue
		}
		rootInfo := RootInfo{
			ID: id,
			Commit: github.Commit{
				Revision: prRevision.Revision,
			},
			Root: root,
			Repo: prRevision.Repo,
		}
		future := p.processRoot(ctx, rootInfo)
		rootInfos = append(rootInfos, rootInfo)
		futures = append(futures, future)
	}
	return p.awaitWorkflows(ctx, rootInfos, futures)
}

func (p *Processor) processRoot(ctx workflow.Context, rootInfo RootInfo) workflow.ChildWorkflowFuture {
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
		DeploymentID: rootInfo.ID.String(),
		Revision:     rootInfo.Commit.Revision,
		WorkflowMode: terraformActivities.PR,
	}
	future := workflow.ExecuteChildWorkflow(ctx, p.TFWorkflow, request)
	return future
}

// awaitWorkflows creates a selector to listen to the completion of each root's child workflow future and any state
// change signals they send over the shared WorkflowStateChangeSignal channel; we only return when all workflows complete
func (p *Processor) awaitWorkflows(ctx workflow.Context, rootInfos []RootInfo, futures []workflow.ChildWorkflowFuture) []activities.PolicySet {
	selector := workflow.NewNamedSelector(ctx, "TerraformChildWorkflow")
	ch := workflow.GetSignalChannel(ctx, state.WorkflowStateChangeSignal)
	workflowsLeft := len(futures)
	for _, rootInfo := range rootInfos {
		selector.AddReceive(ch, func(c workflow.ReceiveChannel, _ bool) {
			p.TFStateReceiver.Receive(ctx, c, rootInfo)
		})
	}

	var failedPolicies []activities.PolicySet
	for _, future := range futures {
		selector.AddFuture(future, func(f workflow.Future) {
			defer func() {
				workflowsLeft--
			}()
			var resp terraform.Response
			if err := f.Get(ctx, &resp); err != nil {
				workflow.GetLogger(workflow.WithValue(ctx, internalContext.ErrKey, err)).Error("executing terraform workflow")
				return
			}
			for _, result := range resp.ValidationResults {
				if result.Status == activities.Fail {
					failedPolicies = append(failedPolicies, result.PolicySet)
				}
			}
		})
	}

	for {
		selector.Select(ctx)
		if workflowsLeft == 0 {
			break
		}
	}
	return failedPolicies
}
