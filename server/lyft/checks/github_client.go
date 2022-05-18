package checks

import (
	"context"
	"fmt"

	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/events/vcs/types"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/lyft/feature"
)

// Wrapping github client with a struct to allow for method resolution
type GithubClient struct {
	*vcs.GithubClient
}

// ChecksClientWrapper uses a wrapped GithubClient to allow for method resoltion
// Usiing vcs.GithubClient directly did not allow to assign a client when initializing ChecksClientWrapper unless it's a named field which is not
// possible since using githubClient as an attribute means this struct does not implement the Client interface.
type ChecksClientWrapper struct {
	GithubClient
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

	// Checks
	return c.GithubClient.UpdateChecksStatus(ctx, request)
}
