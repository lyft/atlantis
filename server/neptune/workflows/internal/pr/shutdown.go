package pr

import (
	"context"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision"
	"go.temporal.io/sdk/workflow"
)

const ClosedState = "closed"

type githubActivities interface {
	GithubGetPullRequestState(ctx context.Context, request activities.GetPullRequestStateRequest) (activities.GetPullRequestStateResponse, error)
}

type ShutdownStateChecker struct {
	GithubActivities githubActivities
	PRNumber         int
}

func (s ShutdownStateChecker) ShouldShutdown(ctx workflow.Context, prRevision revision.Revision) bool {
	req := activities.GetPullRequestStateRequest{
		Repo:     prRevision.Repo,
		PRNumber: s.PRNumber,
	}
	var resp activities.GetPullRequestStateResponse
	err := workflow.ExecuteActivity(ctx, s.GithubActivities.GithubGetPullRequestState, req).Get(ctx, &resp)
	if err != nil {
		workflow.GetLogger(ctx).Error(err.Error())
		return false
	}
	if resp.State == ClosedState {
		workflow.GetLogger(ctx).Info("pr is closed, shutting down")
		return true
	}
	return false
}
