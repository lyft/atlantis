package checks

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/events/vcs/types"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/lyft/feature"
)

// [WENGINES-4643] TODO: Remove this wrapper and add checks implementation to UpdateStatus() directly after github checks is stable
type ChecksClientWrapper struct {
	*vcs.GithubClient
	FeatureAllocator feature.Allocator
	Logger           logging.Logger
}

func (c *ChecksClientWrapper) UpdateStatus(ctx context.Context, request types.UpdateStatusRequest) error {
	shouldAllocate, err := c.FeatureAllocator.ShouldAllocate(feature.GithubChecks, request.Repo.FullName)
	if err != nil {
		c.Logger.ErrorContext(ctx, fmt.Sprintf("unable to allocate for feature: %s", feature.GithubChecks), map[string]interface{}{
			"error": err.Error(),
		})
	}

	if !shouldAllocate {
		return c.GithubClient.UpdateStatus(ctx, request)
	}

	// Get all commit statuses and check if the commit status for this operation is pending
	// and mirror the checks status. This is possible when PRs are in-flight during rollout.
	// [WENGINES-4643] - Clean up after github checks is stable.
	statuses, err := c.GithubClient.GetRepoStatuses(request.Repo, models.PullRequest{
		HeadCommit: request.Ref,
	})
	if err != nil {
		return errors.Wrap(err, "retrieving repo statuses")
	}

	for _, status := range statuses {
		// Skip if name does not match or the state is same
		if *status.Context != request.StatusName || isSameState(*status.State, request.State) {
			continue
		}
		c.GithubClient.UpdateStatus(ctx, request)
	}

	return c.GithubClient.UpdateChecksStatus(ctx, request)
}

func isSameState(statusState string, requestState models.CommitStatus) bool {
	if requestState == models.PendingCommitStatus && statusState == "pending" ||
		requestState == models.FailedCommitStatus && statusState == "failure" ||
		requestState == models.SuccessCommitStatus && statusState == "success" {
		return true
	}
	return false
}
