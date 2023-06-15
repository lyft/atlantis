package event

import (
	"context"
	"github.com/hashicorp/go-multierror"
	"time"

	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/neptune/gateway/config"
	"github.com/runatlantis/atlantis/server/neptune/gateway/pr"
	"github.com/runatlantis/atlantis/server/neptune/gateway/requirement"
	"github.com/runatlantis/atlantis/server/neptune/lyft/feature"
	"github.com/runatlantis/atlantis/server/vcs/provider/github"
	"go.temporal.io/sdk/client"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/legacy/http"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/models"
)

type legacyHandler interface {
	Handle(ctx context.Context, request *http.BufferedRequest, event PullRequest, allRoots []*valid.MergedProjectCfg) error
}

type prSignaler interface {
	SignalWithStartWorkflow(ctx context.Context, rootCfgs []*valid.MergedProjectCfg, prRequest pr.Request) (client.WorkflowRun, error)
}

type ModifiedPullHandler struct {
	Logger             logging.Logger
	Scheduler          scheduler
	RootConfigBuilder  rootConfigBuilder
	GlobalCfg          valid.GlobalCfg
	RequirementChecker requirementChecker
	LegacyHandler      legacyHandler
	Allocator          feature.Allocator
	PRSignaler         prSignaler
}

// PullRequest is our internal representation of a vcs based pr event
type PullRequest struct {
	Pull              models.PullRequest
	User              models.User
	EventType         models.PullRequestEventType
	Timestamp         time.Time
	InstallationToken int64
}

func NewModifiedPullHandler(logger logging.Logger, scheduler scheduler, rootConfigBuilder rootConfigBuilder, globalCfg valid.GlobalCfg, requirementChecker requirementChecker, prSignaler prSignaler, legacyHandler legacyHandler, allocator feature.Allocator) *ModifiedPullHandler {
	return &ModifiedPullHandler{
		Logger:             logger,
		Scheduler:          scheduler,
		RootConfigBuilder:  rootConfigBuilder,
		GlobalCfg:          globalCfg,
		RequirementChecker: requirementChecker,
		LegacyHandler:      legacyHandler,
		PRSignaler:         prSignaler,
		Allocator:          allocator,
	}
}

func (p *ModifiedPullHandler) Handle(ctx context.Context, request *http.BufferedRequest, event PullRequest) error {
	return p.Scheduler.Schedule(ctx, func(ctx context.Context) error {
		return p.handle(ctx, request, event)
	})
}

func (p *ModifiedPullHandler) handle(ctx context.Context, request *http.BufferedRequest, event PullRequest) error {
	criteria := requirement.Criteria{
		User:              event.User,
		Branch:            event.Pull.HeadBranch,
		Repo:              event.Pull.HeadRepo,
		OptionalPull:      &event.Pull,
		InstallationToken: event.InstallationToken,
	}
	if err := p.RequirementChecker.Check(ctx, criteria); err != nil {
		return errors.Wrap(err, "checking pr requirements")
	}

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

	fxns := []func(ctx context.Context, request *http.BufferedRequest, event PullRequest, allRoots []*valid.MergedProjectCfg) error{
		p.LegacyHandler.Handle, // TODO: remove when we deprecate legacy mode
		p.handlePlatformMode,
	}
	var combinedErrors *multierror.Error
	for _, fxn := range fxns {
		f := fxn
		err := p.Scheduler.Schedule(ctx, func(ctx context.Context) error {
			return f(ctx, request, event, rootCfgs)
		})
		combinedErrors = multierror.Append(combinedErrors, err)
	}
	return combinedErrors.ErrorOrNil()
}

func (p *ModifiedPullHandler) handlePlatformMode(ctx context.Context, request *http.BufferedRequest, event PullRequest, roots []*valid.MergedProjectCfg) error {
	// skip signaling workflow if no roots
	if len(roots) == 0 {
		return nil
	}
	// TODO: remove when we begin in-depth testing and rollout of pr mode
	// feature allocator is only temporary while we continue building out implementation
	shouldAllocate, err := p.Allocator.ShouldAllocate(feature.LegacyDeprecation, feature.FeatureContext{
		RepoName: event.Pull.HeadRepo.FullName,
	})
	if err != nil {
		p.Logger.ErrorContext(ctx, "unable to allocate pr mode")
		return nil
	}
	if !shouldAllocate {
		p.Logger.InfoContext(ctx, "handler not configured for allocation")
		return nil
	}
	prRequest := pr.Request{
		Number:            event.Pull.Num,
		Revision:          event.Pull.HeadCommit,
		Repo:              event.Pull.HeadRepo,
		InstallationToken: event.InstallationToken,
		Branch:            event.Pull.HeadBranch,
		// TODO: gate populating field with a config since this is Lyft specific
		ValidateEnvs: buildValidateEnvsFromPull(event),
	}
	run, err := p.PRSignaler.SignalWithStartWorkflow(ctx, roots, prRequest)
	if err != nil {
		return errors.Wrap(err, "signaling workflow")
	}
	p.Logger.InfoContext(ctx, "Signaled workflow.", map[string]interface{}{
		"workflow-id": run.GetID(), "run-id": run.GetRunID(),
	})
	return nil
}

func buildValidateEnvsFromPull(event PullRequest) []pr.ValidateEnvs {
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
