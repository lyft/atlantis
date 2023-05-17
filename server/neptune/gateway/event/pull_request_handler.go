package event

import (
	"context"
	"fmt"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/neptune/gateway/config"
	"github.com/runatlantis/atlantis/server/neptune/gateway/deploy/requirement"
	"github.com/runatlantis/atlantis/server/vcs/provider/github"
	"time"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/http"
	"github.com/runatlantis/atlantis/server/logging"
)

type vcsStatusUpdater interface {
	UpdateCombined(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.VCSStatus, cmdName fmt.Stringer, statusID string, output string) (string, error)
	UpdateCombinedCount(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.VCSStatus, cmdName fmt.Stringer, numSuccess int, numTotal int, statusID string) (string, error)
}

type workerProxy interface {
	Handle(ctx context.Context, request *http.BufferedRequest, event PullRequest) error
}

type ModifiedPullHandler struct {
	WorkerProxy        workerProxy
	Logger             logging.Logger
	Scheduler          scheduler
	RootConfigBuilder  rootConfigBuilder
	GlobalCfg          valid.GlobalCfg
	VCSStatusUpdater   vcsStatusUpdater
	RequirementChecker requirementChecker
}

// PullRequest is our internal representation of a vcs based pr event
type PullRequest struct {
	Pull              models.PullRequest
	User              models.User
	EventType         models.PullRequestEventType
	Timestamp         time.Time
	InstallationToken int64
}

func NewModifiedPullHandler(logger logging.Logger, workerProxy *PullSNSWorkerProxy, scheduler scheduler, rootConfigBuilder rootConfigBuilder, globalCfg valid.GlobalCfg, updater vcsStatusUpdater, requirementChecker requirementChecker) *ModifiedPullHandler {
	return &ModifiedPullHandler{
		WorkerProxy:        workerProxy,
		Logger:             logger,
		Scheduler:          scheduler,
		RootConfigBuilder:  rootConfigBuilder,
		GlobalCfg:          globalCfg,
		VCSStatusUpdater:   updater,
		RequirementChecker: requirementChecker,
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

	// set clone depth to 1 for repos with a branch checkout strategy
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
	if err := p.handleLegacyMode(ctx, request, event, rootCfgs, legacyModeRoots); err != nil {
		return errors.Wrap(err, "handling legacy mode")
	}
	if err := p.handlePlatformMode(ctx, event, platformModeRoots); err != nil {
		return errors.Wrap(err, "handling platform mode")
	}
	return nil
}

func (p *ModifiedPullHandler) handleLegacyMode(ctx context.Context, request *http.BufferedRequest, event PullRequest, allRoots []*valid.MergedProjectCfg, legacyRoots []*valid.MergedProjectCfg) error {
	// mark legacy statuses as successful if there are no roots in general
	// this is processed here to make it easy to clean up when we deprecate legacy mode
	if len(allRoots) == 0 {
		for _, cmd := range []command.Name{command.Plan, command.Apply, command.PolicyCheck} {
			if _, statusErr := p.VCSStatusUpdater.UpdateCombinedCount(ctx, event.Pull.HeadRepo, event.Pull, models.SuccessVCSStatus, cmd, 0, 0, ""); statusErr != nil {
				p.Logger.WarnContext(ctx, fmt.Sprintf("unable to update commit status: %s", statusErr))
			}
		}
		return nil
	}

	// mark apply status as successful if there are no legacy roots
	if len(legacyRoots) == 0 {
		if _, statusErr := p.VCSStatusUpdater.UpdateCombined(ctx, event.Pull.HeadRepo, event.Pull, models.SuccessVCSStatus, command.Apply, "", PlatformModeApplyStatusMessage); statusErr != nil {
			p.Logger.WarnContext(ctx, fmt.Sprintf("unable to update commit status: %s", statusErr))
		}
	}

	// mark plan status as queued
	if _, err := p.VCSStatusUpdater.UpdateCombined(ctx, event.Pull.HeadRepo, event.Pull, models.QueuedVCSStatus, command.Plan, "", "Request received. Adding to the queue..."); err != nil {
		p.Logger.WarnContext(ctx, fmt.Sprintf("unable to update commit status: %s", err))
	}

	// forward to sns
	err := p.WorkerProxy.Handle(ctx, request, event)
	if err != nil {
		return errors.Wrap(err, "proxying request to sns")
	}
	return nil
}

func (p *ModifiedPullHandler) handlePlatformMode(_ context.Context, _ PullRequest, _ []*valid.MergedProjectCfg) error {
	// TODO: handle platform mode
	return nil
}
