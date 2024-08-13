package event

import (
	"context"
	"fmt"

	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/legacy/events/command"
	"github.com/runatlantis/atlantis/server/legacy/http"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/models"
)

const PlatformModeApplyStatusMessage = "THIS IS A LEGACY STATUS CHECK AND IS NOT RELEVANT PLEASE LOOK AT atlantis/deploy status checks"

type vcsStatusUpdater interface {
	UpdateCombined(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.VCSStatus, cmdName fmt.Stringer, statusID string, output string) (string, error)
	UpdateCombinedCount(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.VCSStatus, cmdName fmt.Stringer, numSuccess int, numTotal int, statusID string) (string, error)
}

type LegacyPullHandler struct {
	VCSStatusUpdater vcsStatusUpdater
	Logger           logging.Logger
}

func (l *LegacyPullHandler) Handle(ctx context.Context, request *http.BufferedRequest, event PullRequest, allRoots []*valid.MergedProjectCfg) error {
	// mark legacy statuses as successful if there are no roots in general
	// this is processed here to make it easy to clean up when we deprecate legacy mode
	if len(allRoots) == 0 {
		if _, statusErr := l.VCSStatusUpdater.UpdateCombinedCount(ctx, event.Pull.HeadRepo, event.Pull, models.SuccessVCSStatus, command.Plan, 0, 0, ""); statusErr != nil {
			l.Logger.WarnContext(ctx, fmt.Sprintf("unable to update commit status: %s", statusErr))
		}
		return nil
	}
	return nil
}
