package event

import (
	"bytes"
	"context"
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/neptune/gateway/config"
	"github.com/runatlantis/atlantis/server/vcs/provider/github"
	"time"

	"github.com/runatlantis/atlantis/server/neptune/gateway/pr"
	"github.com/runatlantis/atlantis/server/neptune/lyft/feature"
	"github.com/uber-go/tally/v4"
	"go.temporal.io/api/serviceerror"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/legacy/http"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/models"
)

const (
	Approved         = "approved"
	ChangesRequested = "changes_requested"
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

type workflowSignaler interface {
	SendReviewSignal(ctx context.Context, repoName string, pullNum int, revision string) error
	SendRevisionSignal(ctx context.Context, rootCfgs []*valid.MergedProjectCfg, request pr.Request) error
}

type PullRequestReviewWorkerProxy struct {
	Scheduler         scheduler
	SnsWriter         Writer
	Logger            logging.Logger
	CheckRunFetcher   fetcher
	Allocator         feature.Allocator
	WorkflowSignaler  workflowSignaler
	Scope             tally.Scope
	RootConfigBuilder rootConfigBuilder
	GlobalCfg         valid.GlobalCfg
}

func (p *PullRequestReviewWorkerProxy) Handle(ctx context.Context, event PullRequestReview, request *http.BufferedRequest) error {
	fxns := []func(ctx context.Context, request *http.BufferedRequest, event PullRequestReview) error{
		p.handleLegacyMode,
		p.handlePlatformMode,
	}
	var combinedErrors *multierror.Error
	for _, fxn := range fxns {
		f := fxn
		err := p.Scheduler.Schedule(ctx, func(ctx context.Context) error {
			return f(ctx, request, event)
		})
		combinedErrors = multierror.Append(combinedErrors, err)
	}
	return combinedErrors.ErrorOrNil()
}

func (p *PullRequestReviewWorkerProxy) handleLegacyMode(ctx context.Context, request *http.BufferedRequest, event PullRequestReview) error {
	// Ignore non-approval events
	if event.State != Approved {
		return nil
	}
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
	// Ignore events that are neither approved nor changes requested
	if event.State != Approved && event.State != ChangesRequested {
		return nil
	}
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

	switch event.State {
	case ChangesRequested:
		err = p.handleChangesRequestedEvent(ctx, event)
	case Approved:
		err = p.WorkflowSignaler.SendReviewSignal(ctx, event.Repo.FullName, event.Pull.Num, event.Ref)
	default:
		return nil
	}

	var workflowNotFoundErr *serviceerror.NotFound
	if errors.As(err, &workflowNotFoundErr) {
		// we shouldn't care about approvals for workflows that don't exist
		tags := map[string]string{"repo": event.Pull.HeadRepo.FullName}
		p.Scope.Tagged(tags).Counter("workflow_not_found").Inc(1)
		return nil
	}
	return err
}

func (p *PullRequestReviewWorkerProxy) handleChangesRequestedEvent(ctx context.Context, event PullRequestReview) error {
	// TODO: consider adding check confirming a failing plan check run exists before proceeding
	commit := &config.RepoCommit{
		Repo:          event.Pull.HeadRepo,
		Branch:        event.Pull.HeadBranch,
		Sha:           event.Pull.HeadCommit,
		OptionalPRNum: event.Pull.Num,
	}

	// set clone depth to 1 for repos with a branch checkout strategy,
	// repos with a branch checkout strategy are most likely large and
	// would take too long to provide a full history depth within a clone
	cloneDepth := 0
	matchingRepo := p.GlobalCfg.MatchingRepo(event.Pull.HeadRepo.ID())
	if matchingRepo != nil && matchingRepo.CheckoutStrategy == "branch" {
		cloneDepth = 1
	}
	builderOptions := config.BuilderOptions{
		RepoFetcherOptions: &github.RepoFetcherOptions{
			CloneDepth: cloneDepth,
		},
	}
	rootCfgs, err := p.RootConfigBuilder.Build(ctx, commit, event.InstallationToken, builderOptions)
	if err != nil {
		return errors.Wrap(err, "generating roots")
	}
	prRequest := pr.Request{
		Number:            event.Pull.Num,
		Revision:          event.Pull.HeadCommit,
		Repo:              event.Pull.HeadRepo,
		InstallationToken: event.InstallationToken,
		Branch:            event.Pull.HeadBranch,
		ValidateEnvs:      buildValidateEnvsFromPullReview(event),
	}
	err = p.WorkflowSignaler.SendRevisionSignal(ctx, rootCfgs, prRequest)
	return err
}

func buildValidateEnvsFromPullReview(event PullRequestReview) []pr.ValidateEnvs {
	return []pr.ValidateEnvs{
		{
			Username:       event.User.Username,
			PullNum:        event.Pull.Num,
			PullAuthor:     event.Pull.Author,
			HeadCommit:     event.Pull.HeadCommit,
			BaseBranchName: event.Pull.HeadRepo.DefaultBranch,
		},
	}
}
