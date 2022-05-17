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
	UpdateStatus(ctx context.Context, request types.UpdateStatusRequest) error
}

// Defaults to pull status updater if checks is turned off
type FeatureAwareStatusUpdater struct {
	PullStatusUpdater
	ChecksStatusUpdater

	FeatureAllocator feature.Allocator
	Logger           logging.Logger
}

func (f *FeatureAwareStatusUpdater) UpdateStatus(ctx context.Context, request types.UpdateStatusRequest) error {
	shouldAllocate, err := f.FeatureAllocator.ShouldAllocate(feature.GithubChecks, request.Repo.FullName)
	if err != nil {
		f.Logger.ErrorContext(ctx, fmt.Sprintf("unable to allocate for feature: %s", feature.GithubChecks), map[string]interface{}{
			"error": err.Error(),
		})
	}

	if shouldAllocate {
		return f.ChecksStatusUpdater.UpdateStatus(ctx, request)
	}
	return f.PullStatusUpdater.UpdateStatus(ctx, request)
}

// Used to update status checks in the pull request
type PullStatusUpdater struct {
	Client *github.Client
}

// UpdateStatus updates the status badge on the pull request.
// See https://github.com/blog/1227-commit-status-api.
func (g *PullStatusUpdater) UpdateStatus(ctx context.Context, request types.UpdateStatusRequest) error {
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

	_, _, err := g.Client.Repositories.CreateStatus(ctx, request.Repo.Owner, request.Repo.Name, request.Ref, status)
	return err
}

// Used to update github checks in the pull request
type ChecksStatusUpdater struct {
	Client *github.Client
}

func (c *ChecksStatusUpdater) UpdateStatus(ctx context.Context, request types.UpdateStatusRequest) error {
	// TODO: Implement updating github checks
	// - Get all checkruns for this SHA
	// - Match the UpdateReqIdentifier with the check run. If it exists, update the checkrun. If it does not, create a new check run.

	// Checks uses Status and Conlusion. Need to map models.CommitStatus to Status and Conclusion
	// Status -> queued, in_progress, completed
	// Conclusion -> failure, neutral, cancelled, timed_out, or action_required. (Optional. Required if you provide a status of "completed".)
	return nil
}
