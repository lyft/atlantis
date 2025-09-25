package pr

import (
	"strconv"
	"time"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	workflowMetrics "github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision"
	"go.temporal.io/sdk/workflow"
)

const TaskQueue = "pr"

type prActivities struct {
	*activities.Github
}

func Workflow(ctx workflow.Context, request Request, tfWorkflow revision.TFWorkflow) error {
	options := workflow.ActivityOptions{
		TaskQueue:           TaskQueue,
		StartToCloseTimeout: 5 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, options)
	scope := workflowMetrics.NewScope(ctx).SubScopeWithTags(map[string]string{
		"repo":   request.RepoFullName,
		"pr-num": strconv.Itoa(request.PRNum),
	})
	runner := newRunner(ctx, scope, request.Organization, tfWorkflow, request.PRNum)
	return runner.Run(ctx)
}
