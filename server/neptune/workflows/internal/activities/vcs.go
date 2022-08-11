package activities

import (
	"context"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
)

// TODO: Initial implementation will support GH VCS, but we will need to eventually abstract to other platforms

type vcsActivities struct{}

// Github Clone

type GithubRepoCloneRequest struct {
	Repo           github.Repo
	Revision       string
	DestinationDir string
}

func (t *terraformActivities) GithubRepoClone(ctx context.Context, request GithubRepoCloneRequest) error {
	return nil
}
