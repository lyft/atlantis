package rebase

import (
	"time"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	workflowMetrics "github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const RebaseTaskQueue = "rebase"

type Request struct {
	Repo github.Repo
	Root terraform.Root
}

type rebaseActivities struct {
	*activities.RevsionSetter
	*activities.Github
}

func Workflow(ctx workflow.Context, request Request) error {
	// temporal effectively "injects" this, it just cares about the method names,
	// so we're modeling our own DI around this.
	var r *rebaseActivities
	scope := workflowMetrics.NewScope(ctx)

	// GH API calls should not hit ratelimit issues since we cap the max number of GH API calls to 7k which is well within our budget
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	})

	pullRebaser := PullRebaser{
		RebaseActivites: *r,
	}

	return pullRebaser.RebaseOpenPRsForRoot(ctx, request.Repo, request.Root, scope)
}
