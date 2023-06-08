package event

import (
	"context"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/http"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/lyft/feature"
	"github.com/runatlantis/atlantis/server/neptune/gateway/pr"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"github.com/uber-go/tally/v4"
	"go.temporal.io/api/serviceerror"
)

type prCloseSignaler interface {
	SignalWorkflow(ctx context.Context, workflowID string, runID string, signalName string, arg interface{}) error
}

type ClosedPullRequestHandler struct {
	WorkerProxy     workerProxy
	Allocator       feature.Allocator
	Logger          logging.Logger
	PRCloseSignaler prCloseSignaler
	Scope           tally.Scope
}

func (c *ClosedPullRequestHandler) Handle(ctx context.Context, request *http.BufferedRequest, event PullRequest) error {
	if err := c.WorkerProxy.Handle(ctx, request, event); err != nil {
		c.Logger.ErrorContext(ctx, err.Error())
	}

	if err := c.handlePlatformMode(ctx, event); err != nil {
		return errors.Wrap(err, "handling platform mode")
	}
	return nil
}

func (c *ClosedPullRequestHandler) handlePlatformMode(ctx context.Context, event PullRequest) error {
	shouldAllocate, err := c.Allocator.ShouldAllocate(feature.LegacyDeprecation, feature.FeatureContext{
		RepoName: event.Pull.HeadRepo.FullName,
	})
	if err != nil {
		c.Logger.ErrorContext(ctx, "unable to allocate pr mode")
		return nil
	}
	if !shouldAllocate {
		c.Logger.InfoContext(ctx, "handler not configured for allocation")
		return nil
	}
	err = c.PRCloseSignaler.SignalWorkflow(
		ctx,
		pr.BuildPRWorkflowID(event.Pull.HeadRepo.FullName, event.Pull.Num),
		// keeping this empty is fine since temporal will find the currently running workflow
		"",
		workflows.PRShutdownSignalName,
		workflows.PRShutdownRequest{})

	var workflowNotFoundErr *serviceerror.NotFound
	if errors.As(err, &workflowNotFoundErr) {
		// we shouldn't care about closing workflows that don't exist
		tags := map[string]string{"repo": event.Pull.HeadRepo.FullName}
		c.Scope.Tagged(tags).Counter("workflow_not_found").Inc(1)
		return nil
	}
	return err
}
