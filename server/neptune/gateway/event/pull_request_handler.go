package event

import (
	"context"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/neptune/gateway/config"
	"github.com/runatlantis/atlantis/server/neptune/gateway/requirement"
	"github.com/runatlantis/atlantis/server/vcs/provider/github"
	"time"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/http"
	"github.com/runatlantis/atlantis/server/logging"
)

type legacyHandler interface {
	Handle(ctx context.Context, request *http.BufferedRequest, event PullRequest, allRoots []*valid.MergedProjectCfg, legacyRoots []*valid.MergedProjectCfg) error
}

type ModifiedPullHandler struct {
	Logger             logging.Logger
	Scheduler          scheduler
	RootConfigBuilder  rootConfigBuilder
	GlobalCfg          valid.GlobalCfg
	RequirementChecker requirementChecker
	LegacyHandler      legacyHandler
}

// PullRequest is our internal representation of a vcs based pr event
type PullRequest struct {
	Pull              models.PullRequest
	User              models.User
	EventType         models.PullRequestEventType
	Timestamp         time.Time
	InstallationToken int64
}

func NewModifiedPullHandler(logger logging.Logger, scheduler scheduler, rootConfigBuilder rootConfigBuilder, globalCfg valid.GlobalCfg, requirementChecker requirementChecker, legacyHandler legacyHandler) *ModifiedPullHandler {
	return &ModifiedPullHandler{
		Logger:             logger,
		Scheduler:          scheduler,
		RootConfigBuilder:  rootConfigBuilder,
		GlobalCfg:          globalCfg,
		RequirementChecker: requirementChecker,
		LegacyHandler:      legacyHandler,
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

	var legacyModeRoots []*valid.MergedProjectCfg
	var platformModeRoots []*valid.MergedProjectCfg
	for _, rootCfg := range rootCfgs {
		if rootCfg.WorkflowMode == valid.PlatformWorkflowMode {
			platformModeRoots = append(platformModeRoots, rootCfg)
		} else {
			legacyModeRoots = append(legacyModeRoots, rootCfg)
		}
	}

	// TODO: remove when we deprecate legacy mode
	if err := p.LegacyHandler.Handle(ctx, request, event, rootCfgs, legacyModeRoots); err != nil {
		p.Logger.ErrorContext(ctx, err.Error())
	}
	if err := p.handlePlatformMode(ctx, event, platformModeRoots); err != nil {
		return errors.Wrap(err, "handling platform mode")
	}
	return nil
}

func (p *ModifiedPullHandler) handlePlatformMode(_ context.Context, _ PullRequest, _ []*valid.MergedProjectCfg) error {
	// TODO: handle platform mode
	return nil
}
