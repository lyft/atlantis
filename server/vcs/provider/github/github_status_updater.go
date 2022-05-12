package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v31/github"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs/types"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/lyft/feature"
)

// Interface to support status updates for PR Status Checks and Github Status Checks
type StatusUpdater interface {
	UpdateStatus(ctx context.Context, request types.UpdateStatusRequest) (string, error)
}

// Defaults to pull status updater if checks is turned off
type FeatureAwareStatusUpdater struct {
	Pull             StatusUpdater
	Check            StatusUpdater
	FeatureAllocator feature.Allocator

	// TODO: Replace with Logger
	Logger logging.SimpleLogging
}

func (f *FeatureAwareStatusUpdater) UpdateStatus(ctx context.Context, request types.UpdateStatusRequest) (string, error) {
	shouldAllocate, err := f.FeatureAllocator.ShouldAllocate(feature.GithubChecks, "")
	if err != nil {
		f.Logger.Errorf("unable to allocate for feature: %s, error: %s", feature.LogPersistence, err)
	}

	if shouldAllocate {
		return f.Check.UpdateStatus(ctx, request)
	}
	return f.Pull.UpdateStatus(ctx, request)
}

type PullStatusUpdater struct {
	Client *github.Client
}

// UpdateStatus updates the status badge on the pull request.
// See https://github.com/blog/1227-commit-status-api.
func (g *PullStatusUpdater) UpdateStatus(ctx context.Context, request types.UpdateStatusRequest) (string, error) {
	ghState := "error"
	switch request.State {
	case models.PendingCommitStatus:
		ghState = "pending"
	case models.SuccessCommitStatus:
		ghState = "success"
	case models.FailedCommitStatus:
		ghState = "failure"
	}

	status := &github.RepoStatus{
		State:       github.String(ghState),
		Description: github.String(request.Description),
		Context:     github.String(request.StatusName),
		TargetURL:   &request.DetailsURL,
	}

	// TODO: For debug purposes, remove when implementing github checks
	if request.StatusId == "" {
		_, _, err := g.Client.Repositories.CreateStatus(ctx, request.Repo.Owner, request.Repo.Name, request.Ref, status)
		fmt.Printf("DEBUG: Creating status check: %v", status)
		return "1", err
	} else {
		fmt.Printf("DEBUG: Updating status check for: %v", status)
		_, _, err := g.Client.Repositories.CreateStatus(ctx, request.Repo.Owner, request.Repo.Name, request.Ref, status)
		return request.StatusId, err
	}
}

type ChecksStatusUpdater struct {
	Client *github.Client
}

func (c *ChecksStatusUpdater) UpdateStatus(ctx context.Context, request types.UpdateStatusRequest) (string, error) {
	// TODO: Implement update status Github Checks
	return "", nil
}
