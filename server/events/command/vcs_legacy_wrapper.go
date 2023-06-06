package command

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/lyft/feature"
)

type delegate interface {
	UpdateCombined(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.VCSStatus, cmdName fmt.Stringer, statusID string, output string) (string, error)
	UpdateCombinedCount(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.VCSStatus, cmdName fmt.Stringer, numSuccess int, numTotal int, statusID string) (string, error)
	UpdateProject(ctx context.Context, projectCtx ProjectContext, cmdName fmt.Stringer, status models.VCSStatus, url string, statusID string) (string, error)
}
type LegacyDeprecationVCSStatusUpdater struct {
	Delegate  delegate
	Allocator feature.Allocator
	Logger    logging.Logger
}

func (l *LegacyDeprecationVCSStatusUpdater) UpdateCombined(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.VCSStatus, cmdName fmt.Stringer, statusID string, output string) (string, error) {
	shouldAllocate, err := l.Allocator.ShouldAllocate(feature.LegacyDeprecation, feature.FeatureContext{
		RepoName: repo.FullName,
	})
	if err != nil {
		return "", errors.Wrap(err, "unable to allocate legacy deprecation feature flag")
	}
	// if legacy deprecation is enabled, don't mutate check runs in legacy workflow
	if shouldAllocate {
		l.Logger.InfoContext(ctx, "legacy deprecation feature flag enabled, not updating check runs")
		return "", nil
	}
	return l.Delegate.UpdateCombined(ctx, repo, pull, status, cmdName, statusID, output)
}

func (l *LegacyDeprecationVCSStatusUpdater) UpdateCombinedCount(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.VCSStatus, cmdName fmt.Stringer, numSuccess int, numTotal int, statusID string) (string, error) {
	shouldAllocate, err := l.Allocator.ShouldAllocate(feature.LegacyDeprecation, feature.FeatureContext{
		RepoName: repo.FullName,
	})
	if err != nil {
		return "", errors.Wrap(err, "unable to allocate legacy deprecation feature flag")
	}
	// if legacy deprecation is enabled, don't mutate check runs in legacy workflow
	if shouldAllocate {
		l.Logger.InfoContext(ctx, "legacy deprecation feature flag enabled, not updating check runs")
		return "", nil
	}
	return l.Delegate.UpdateCombinedCount(ctx, repo, pull, status, cmdName, numSuccess, numTotal, statusID)
}

func (l *LegacyDeprecationVCSStatusUpdater) UpdateProject(ctx context.Context, projectCtx ProjectContext, cmdName fmt.Stringer, status models.VCSStatus, url string, statusID string) (string, error) {
	shouldAllocate, err := l.Allocator.ShouldAllocate(feature.LegacyDeprecation, feature.FeatureContext{
		RepoName: projectCtx.HeadRepo.FullName,
	})
	if err != nil {
		return "", errors.Wrap(err, "unable to allocate legacy deprecation feature flag")
	}
	// if legacy deprecation is enabled, don't mutate check runs in legacy workflow
	if shouldAllocate {
		l.Logger.InfoContext(ctx, "legacy deprecation feature flag enabled, not updating project check run")
		return "", nil
	}
	return l.Delegate.UpdateProject(ctx, projectCtx, cmdName, status, url, statusID)
}
