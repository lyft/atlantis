package executor

import (
	"github.com/runatlantis/atlantis/server/neptune/lyft/workflows"
	"github.com/runatlantis/atlantis/server/neptune/workflows/plugins"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/workflow"
)

type PRRevisionWorkflowExecutor struct {
	TaskQueue string
}

func (p *PRRevisionWorkflowExecutor) Execute(ctx workflow.Context, deployment plugins.TerraformDeploymentInfo) error {
	// Let's only execute this workflow if we're on the default branch
	if deployment.Commit.Branch != deployment.Repo.DefaultBranch {
		return nil
	}

	ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		TaskQueue: p.TaskQueue,

		// configuring this ensures the child workflow will continue execution when the parent workflow terminates
		ParentClosePolicy: enums.PARENT_CLOSE_POLICY_ABANDON,
	})

	future := workflow.ExecuteChildWorkflow(ctx, workflows.PRRevision, workflows.PRRevisionRevisionRequest{
		Repo:     deployment.Repo,
		Root:     deployment.Root,
		Revision: deployment.Commit.Revision,
	})

	return future.GetChildWorkflowExecution().Get(ctx, nil)
}
