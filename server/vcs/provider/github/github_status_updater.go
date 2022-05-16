package github

import (
	"context"

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
	Pull             StatusUpdater
	Check            StatusUpdater
	FeatureAllocator feature.Allocator

	// TODO: Replace with Logger
	Logger logging.SimpleLogging
}

func (f *FeatureAwareStatusUpdater) UpdateStatus(ctx context.Context, request types.UpdateStatusRequest) error {
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

type ChecksStatusUpdater struct {
	Client *github.Client
}

// status -> queued, in_progress, completed
// "failure", "neutral", "cancelled", "timed_out", or "action_required". (Optional. Required if you provide a status of "completed".)
func (c *ChecksStatusUpdater) UpdateStatus(ctx context.Context, request types.UpdateStatusRequest) error {

	status := "queued"
	conclusion := ""

	// TODO: Fix status of checks
	switch request.State {
	case models.SuccessCommitStatus:
		status = "completed"
		conclusion = "success"

	case models.PendingCommitStatus:
		status = "in_progress"

	case models.FailedCommitStatus:
		status = "completed"
		conclusion = "failure"
	}

	result, _, err := c.Client.Checks.ListCheckRunsForRef(ctx, request.Repo.Owner, request.Repo.Name, request.Ref, &github.ListCheckRunsOptions{})
	if err != nil {
		return err
	}

	for _, checkRun := range result.CheckRuns {
		// Update status check if checkRun exists
		if DoesCheckRunExist(*checkRun, request) {
			updateCheckRunOpts := github.UpdateCheckRunOptions{
				Name:    request.StatusName,
				HeadSHA: &request.Ref,
				Status:  &status,
			}

			if request.DetailsURL != "" {
				updateCheckRunOpts.DetailsURL = &request.DetailsURL
			}

			if request.Description != "" {
				updateCheckRunOpts.Output = &github.CheckRunOutput{
					Title:   &request.StatusName,
					Summary: &request.Description,
				}
			}

			// Add conclusion if not pending state
			if request.State != models.PendingCommitStatus {
				updateCheckRunOpts.Conclusion = &conclusion
			}

			_, _, err := c.Client.Checks.UpdateCheckRun(ctx, request.Repo.Owner, request.Repo.Name, *checkRun.ID, updateCheckRunOpts)
			return err
		}
	}

	// Create check run if dne
	createCheckRunOpts := github.CreateCheckRunOptions{
		Name:    request.StatusName,
		HeadSHA: request.Ref,
		Status:  &status,
	}

	if request.DetailsURL != "" {
		createCheckRunOpts.DetailsURL = &request.DetailsURL
	}

	if request.Description != "" {
		createCheckRunOpts.Output = &github.CheckRunOutput{
			Title:   &request.StatusName,
			Summary: &request.Description,
		}
	}

	// Add conclusion if not pending state
	if request.State != models.PendingCommitStatus {
		createCheckRunOpts.Conclusion = &conclusion
	}

	_, _, err = c.Client.Checks.CreateCheckRun(ctx, request.Repo.Owner, request.Repo.Name, createCheckRunOpts)
	if err != nil {
		return err
	}

	return nil
}

func DoesCheckRunExist(checkRun github.CheckRun, updateRequest types.UpdateStatusRequest) bool {
	return *checkRun.App.Owner.Login == updateRequest.Repo.Owner && *checkRun.HeadSHA == updateRequest.Ref && *checkRun.Name == updateRequest.StatusName
}
