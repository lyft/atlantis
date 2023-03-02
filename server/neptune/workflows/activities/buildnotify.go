package activities

import (
	"context"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/deployment"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
)

type buildNotifyActivities struct{} // nolint: unused

type BuildNotifyRebasePRRequest struct {
	Repository  deployment.Repo
	PullRequest github.PullRequest
}

type BuildNotifyRebasePRResponse struct {
}

func (b *buildNotifyActivities) BuildNotifyRebasePR(ctx context.Context, request BuildNotifyRebasePRRequest) (BuildNotifyRebasePRResponse, error) { // nolint: unused
	return BuildNotifyRebasePRResponse{}, nil
}
