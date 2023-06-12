package event

import (
	"bytes"
	"context"
	"github.com/runatlantis/atlantis/server/lyft/feature"
	"github.com/runatlantis/atlantis/server/neptune/gateway/pr"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"github.com/uber-go/tally/v4"
	"go.temporal.io/api/serviceerror"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/http"
	"github.com/runatlantis/atlantis/server/logging"
)

const (
	Approved = "approved"
)

type PullRequestReview struct {
	InstallationToken int64
	Repo              models.Repo
	User              models.User
	State             string
	Ref               string
	Timestamp         time.Time
	Pull              models.PullRequest
}

type fetcher interface {
	ListFailedPolicyCheckRunNames(ctx context.Context, installationToken int64, repo models.Repo, ref string) ([]string, error)
}

type prApprovalSignaler interface {
	SignalWorkflow(ctx context.Context, workflowID string, runID string, signalName string, arg interface{}) error
}

type PullRequestReviewWorkerProxy struct {
	Scheduler          scheduler
	SnsWriter          Writer
	Logger             logging.Logger
	CheckRunFetcher    fetcher
	Allocator          feature.Allocator
	PRApprovalSignaler prApprovalSignaler
	Scope              tally.Scope
}

func (p *PullRequestReviewWorkerProxy) Handle(ctx context.Context, event PullRequestReview, request *http.BufferedRequest) error {
	// Ignore non-approval events
	if event.State != Approved {
		return nil
	}
	fxns := []func(ctx context.Context, request *http.BufferedRequest, event PullRequestReview) error{
		p.handleLegacyMode,
		p.handlePlatformMode,
	}
	var combinedErrors *multierror.Error
	for _, f := range fxns {
		err := p.Scheduler.Schedule(ctx, func(ctx context.Context) error {
			return f(ctx, request, event)
		})
		combinedErrors = multierror.Append(combinedErrors, err)
	}
	return combinedErrors.ErrorOrNil()
}

func (p *PullRequestReviewWorkerProxy) handleLegacyMode(ctx context.Context, request *http.BufferedRequest, event PullRequestReview) error {
	// Ignore PRs without failing policy checks
	failedPolicyCheckRuns, err := p.CheckRunFetcher.ListFailedPolicyCheckRunNames(ctx, event.InstallationToken, event.Repo, event.Ref)
	if err != nil {
		p.Logger.ErrorContext(ctx, "unable to list failed policy check runs")
		return err
	}
	if len(failedPolicyCheckRuns) == 0 {
		return nil
	}
	// Forward to SNS to further process in the worker
	return p.forwardToSns(ctx, request)
}

func (p *PullRequestReviewWorkerProxy) forwardToSns(ctx context.Context, request *http.BufferedRequest) error {
	buffer := bytes.NewBuffer([]byte{})
	if err := request.GetRequestWithContext(ctx).Write(buffer); err != nil {
		return errors.Wrap(err, "writing request to buffer")
	}

	if err := p.SnsWriter.WriteWithContext(ctx, buffer.Bytes()); err != nil {
		return errors.Wrap(err, "writing buffer to sns")
	}
	p.Logger.InfoContext(ctx, "proxied request to sns")
	return nil
}

func (p *PullRequestReviewWorkerProxy) handlePlatformMode(ctx context.Context, request *http.BufferedRequest, event PullRequestReview) error {
	shouldAllocate, err := p.Allocator.ShouldAllocate(feature.LegacyDeprecation, feature.FeatureContext{
		RepoName: event.Repo.FullName,
	})
	if err != nil {
		p.Logger.ErrorContext(ctx, "unable to allocate pr mode")
		return nil
	}
	if !shouldAllocate {
		p.Logger.InfoContext(ctx, "prr handler not configured for allocation")
		return nil
	}

	err = p.PRApprovalSignaler.SignalWorkflow(
		ctx,
		pr.BuildPRWorkflowID(event.Repo.FullName, event.Pull.Num),
		// keeping this empty is fine since temporal will find the currently running workflow
		"",
		workflows.PRApprovalSignalName,
		workflows.PRApprovalRequest{Revision: event.Ref})

	var workflowNotFoundErr *serviceerror.NotFound
	if errors.As(err, &workflowNotFoundErr) {
		// we shouldn't care about approvals for workflows that don't exist
		tags := map[string]string{"repo": event.Pull.HeadRepo.FullName}
		p.Scope.Tagged(tags).Counter("workflow_not_found").Inc(1)
		return nil
	}
	return err
}
