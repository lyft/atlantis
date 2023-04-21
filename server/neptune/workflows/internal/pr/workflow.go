package pr

import (
	internalContext "github.com/runatlantis/atlantis/server/neptune/context"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	tfModel "github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	workflowMetrics "github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/notifier"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/terraform"
	"go.temporal.io/sdk/workflow"
	"strconv"
	"time"
)

const TaskQueue = "pr"

type prActivities struct {
	*activities.Github
}

func Workflow(ctx workflow.Context, request Request, tfWorkflow terraform.Workflow) error {
	var a *prActivities
	options := workflow.ActivityOptions{
		TaskQueue:           TaskQueue,
		StartToCloseTimeout: 5 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, options)
	ctx = workflow.WithValue(ctx, internalContext.RepositoryKey, request.RepoFullName)
	ctx = workflow.WithValue(ctx, internalContext.PullNumKey, request.PRNum)
	scope := workflowMetrics.NewScope(ctx).SubScopeWithTags(map[string]string{
		"repo":   request.RepoFullName,
		"pr-num": strconv.Itoa(request.PRNum),
	})
	checkRunCache := notifier.NewGithubCheckRunCache(a)
	notifiers := []terraform.WorkflowNotifier{
		&notifier.CheckRunNotifier{
			CheckRunSessionCache: checkRunCache,
			Mode:                 tfModel.PR,
		},
	}
	tfWorkflowRunner := terraform.NewWorkflowRunner(tfWorkflow, notifiers)
	runner := newRunner(ctx, scope, tfWorkflowRunner)
	return runner.Run(ctx)
}
