package checks

import (
	"context"
	"fmt"
	"strings"

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

// [WENGINES-4643] - Clean up after github checks is stable.
func (c *ChecksClientWrapper) UpdateStatus(ctx context.Context, request types.UpdateStatusRequest) error {

	// UseGithubChecks is set by outputUpdater if feature flag and repo status has already been evaluated.
	// false by default, check the feature flag and existing statuses if this flag is not set.
	if request.UseGithubChecks {
		return c.GithubClient.UpdateChecksStatus(ctx, request)
	}

	shouldAllocate, err := c.FeatureAllocator.ShouldAllocate(feature.GithubChecks, request.Repo.FullName)
	if err != nil {
		c.Logger.ErrorContext(ctx, fmt.Sprintf("unable to allocate for feature: %s", feature.GithubChecks), map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Do not update status if this op fails bc this can cause mix of statuses
	atlantisStatusExists, err := c.doesAtlantisStatusExist(request)
	if err != nil {
		c.Logger.ErrorContext(ctx, err.Error(), map[string]interface{}{})
		return err
	}

	if shouldAllocate && !atlantisStatusExists {
		return c.GithubClient.UpdateChecksStatus(ctx, request)
	}

	return c.GithubClient.UpdateStatus(ctx, request)
}

// Get all commit statuses and check if it already has an atlantis commit status; use commit statuses if it exists
// If not, it means this PR was created after checks was rolled out so, we use github checks for status updates
func (c *ChecksClientWrapper) doesAtlantisStatusExist(request types.UpdateStatusRequest) (bool, error) {
	statuses, err := c.GithubClient.GetRepoStatuses(request.Repo, models.PullRequest{
		HeadCommit: request.Ref,
	})
	if err != nil {
		return false, errors.Wrap(err, "retrieving repo statuses")
	}

	for _, status := range statuses {
		if strings.Contains(*status.Context, "atlantis") {
			return true, nil
		}
	}

	return false, nil
}
