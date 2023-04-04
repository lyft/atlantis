package github

import (
	"context"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"go.temporal.io/sdk/workflow"
)

type githubActivities interface {
	GithubListPRs(ctx context.Context, request activities.ListPRsRequest) (activities.ListPRsResponse, error)
	GithubListModifiedFiles(ctx context.Context, request activities.ListModifiedFilesRequest) (activities.ListModifiedFilesResponse, error)
}

// abstracts configuring the TQ for testing
type ActivityExecutor struct {
	GithubActivities githubActivities
}

func (a *ActivityExecutor) GithubListModifiedFiles(ctx workflow.Context, taskqueue string, request activities.ListModifiedFilesRequest) workflow.Future {
	opts := workflow.GetActivityOptions(ctx)
	opts.TaskQueue = taskqueue
	ctx = workflow.WithActivityOptions(ctx, opts)

	return workflow.ExecuteActivity(ctx, a.GithubActivities.GithubListModifiedFiles, request)
}

func (a *ActivityExecutor) GithubListPRs(ctx workflow.Context, request activities.ListPRsRequest) workflow.Future {
	return workflow.ExecuteActivity(ctx, a.GithubActivities.GithubListPRs, request)
}
