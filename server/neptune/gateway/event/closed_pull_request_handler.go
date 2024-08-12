package event

import (
	"context"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/legacy/http"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/uber-go/tally/v4"
	"go.temporal.io/api/serviceerror"
)

type prCloseSignaler interface {
	SendCloseSignal(ctx context.Context, repoName string, pullNum int) error
}

type ClosedPullRequestHandler struct {
	Logger          logging.Logger
	PRCloseSignaler prCloseSignaler
	Scope           tally.Scope
}

func (c *ClosedPullRequestHandler) Handle(ctx context.Context, request *http.BufferedRequest, event PullRequest) error {
	if err := c.handlePlatformMode(ctx, event); err != nil {
		return errors.Wrap(err, "handling platform mode")
	}
	return nil
}

func (c *ClosedPullRequestHandler) handlePlatformMode(ctx context.Context, event PullRequest) error {
	err := c.PRCloseSignaler.SendCloseSignal(ctx, event.Pull.HeadRepo.FullName, event.Pull.Num)

	var workflowNotFoundErr *serviceerror.NotFound
	if errors.As(err, &workflowNotFoundErr) {
		// we shouldn't care about closing workflows that don't exist
		tags := map[string]string{"repo": event.Pull.HeadRepo.FullName}
		c.Scope.Tagged(tags).Counter("workflow_not_found").Inc(1)
		return nil
	}
	return err
}
