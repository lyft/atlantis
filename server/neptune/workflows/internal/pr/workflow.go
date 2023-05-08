package pr

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	tfModel "github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	workflowMetrics "github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/notifier"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision"
	"go.temporal.io/sdk/workflow"
	"strconv"
	"time"
)

const TaskQueue = "pr"

type prActivities struct {
	*activities.Github
}

func Workflow(ctx workflow.Context, request Request, tfWorkflow revision.TFWorkflow) error {
	var a *prActivities
	options := workflow.ActivityOptions{
		TaskQueue:           TaskQueue,
		StartToCloseTimeout: 5 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, options)
	scope := workflowMetrics.NewScope(ctx).SubScopeWithTags(map[string]string{
		"repo":   request.RepoFullName,
		"pr-num": strconv.Itoa(request.PRNum),
	})
	checkRunCache := notifier.NewGithubCheckRunCache(a)
	notifiers := []revision.WorkflowNotifier{
		&notifier.CheckRunNotifier{
			CheckRunSessionCache: checkRunCache,
			Mode:                 tfModel.PR,
		},
	}
	runner := newRunner(ctx, scope, tfWorkflow, notifiers)
	return runner.Run(ctx)
}
