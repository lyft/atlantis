package event

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/http"
	"github.com/runatlantis/atlantis/server/logging"
)

const PlatformModeApplyStatusMessage = "Bypassed for platform mode"

type vcsStatusUpdater interface {
	UpdateCombined(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.VCSStatus, cmdName fmt.Stringer, statusID string, output string) (string, error)
	UpdateCombinedCount(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.VCSStatus, cmdName fmt.Stringer, numSuccess int, numTotal int, statusID string) (string, error)
}

type workerProxy interface {
	Handle(ctx context.Context, request *http.BufferedRequest, event PullRequest) error
}

type LegacyPullHandler struct {
	VCSStatusUpdater vcsStatusUpdater
	WorkerProxy      workerProxy
	Logger           logging.Logger
}

func (l *LegacyPullHandler) Handle(ctx context.Context, request *http.BufferedRequest, event PullRequest, allRoots []*valid.MergedProjectCfg) error {
	// mark legacy statuses as successful if there are no roots in general
	// this is processed here to make it easy to clean up when we deprecate legacy mode
	if len(allRoots) == 0 {
		for _, cmd := range []command.Name{command.Plan, command.Apply, command.PolicyCheck} {
			if _, statusErr := l.VCSStatusUpdater.UpdateCombinedCount(ctx, event.Pull.HeadRepo, event.Pull, models.SuccessVCSStatus, cmd, 0, 0, ""); statusErr != nil {
				l.Logger.WarnContext(ctx, fmt.Sprintf("unable to update commit status: %s", statusErr))
			}
		}
		return nil
	}

	// mark apply status as successful until we're able to remove this as a required check from our github org.
	if _, statusErr := l.VCSStatusUpdater.UpdateCombined(ctx, event.Pull.HeadRepo, event.Pull, models.SuccessVCSStatus, command.Apply, "", PlatformModeApplyStatusMessage); statusErr != nil {
		l.Logger.WarnContext(ctx, fmt.Sprintf("unable to update commit status: %s", statusErr))
	}

	// mark plan status as queued. since this is the pull handler, we know that we're only executing plans
	if _, err := l.VCSStatusUpdater.UpdateCombined(ctx, event.Pull.HeadRepo, event.Pull, models.QueuedVCSStatus, command.Plan, "", "Request received. Adding to the queue..."); err != nil {
		l.Logger.WarnContext(ctx, fmt.Sprintf("unable to update commit status: %s", err))
	}

	// forward to sns
	err := l.WorkerProxy.Handle(ctx, request, event)
	if err != nil {
		return errors.Wrap(err, "proxying request to sns")
	}
	return nil
}
