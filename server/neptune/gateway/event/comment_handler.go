package event

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/runatlantis/atlantis/server/lyft/feature"
	"github.com/runatlantis/atlantis/server/neptune/gateway/pr"
	"time"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/http"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/neptune/gateway/config"
	"github.com/runatlantis/atlantis/server/neptune/gateway/deploy"
	"github.com/runatlantis/atlantis/server/neptune/gateway/requirement"
	"github.com/runatlantis/atlantis/server/neptune/sync"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
)

const warningMessage = "⚠️ WARNING ⚠️\n\n You are force applying changes from your PR instead of merging into your default branch 🚀. This can have unpredictable consequences 🙏🏽 and should only be used in an emergency 🆘.\n\n To confirm behavior, review and confirm the plan within the generated atlantis/deploy GH check below.\n\n 𝐓𝐡𝐢𝐬 𝐚𝐜𝐭𝐢𝐨𝐧 𝐰𝐢𝐥𝐥 𝐛𝐞 𝐚𝐮𝐝𝐢𝐭𝐞𝐝.\n"

type statusUpdater interface {
	UpdateCombined(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.VCSStatus, cmdName fmt.Stringer, statusID string, output string) (string, error)
}

type commentCreator interface {
	CreateComment(repo models.Repo, pullNum int, comment string, command string) error
}

type rootConfigBuilder interface {
	Build(ctx context.Context, commit *config.RepoCommit, installationToken int64, opts ...config.BuilderOptions) ([]*valid.MergedProjectCfg, error)
}

type requirementChecker interface {
	Check(ctx context.Context, criteria requirement.Criteria) error
}

type errorHandler interface {
	WrapWithHandling(ctx context.Context, event PREvent, commandName string, executor sync.Executor) sync.Executor
}

// Comment is our internal representation of a vcs based comment event.
type Comment struct {
	Pull              models.PullRequest
	BaseRepo          models.Repo
	HeadRepo          models.Repo
	User              models.User
	PullNum           int
	Comment           string
	VCSHost           models.VCSHostType
	Timestamp         time.Time
	InstallationToken int64
}

func (c Comment) GetPullNum() int {
	return c.PullNum
}

func (c Comment) GetInstallationToken() int64 {
	return c.InstallationToken
}

func (c Comment) GetRepo() models.Repo {
	return c.BaseRepo
}

func NewCommentEventWorkerProxy(logger logging.Logger, snsWriter Writer, scheduler scheduler, allocator feature.Allocator, prSignaler prSignaler, deploySignaler deploySignaler, commentCreator commentCreator, vcsStatusUpdater statusUpdater, globalCfg valid.GlobalCfg, rootConfigBuilder rootConfigBuilder, errorHandler errorHandler, requirementChecker requirementChecker) *CommentEventWorkerProxy {
	return &CommentEventWorkerProxy{
		logger:    logger,
		scheduler: scheduler,
		legacyHandler: &LegacyCommentHandler{
			logger:           logger,
			snsWriter:        snsWriter,
			vcsStatusUpdater: vcsStatusUpdater,
			globalCfg:        globalCfg,
		},
		neptuneWorkerProxy: &NeptuneWorkerProxy{
			logger:             logger,
			deploySignaler:     deploySignaler,
			commentCreator:     commentCreator,
			requirementChecker: requirementChecker,
			allocator:          allocator,
			prSignaler:         prSignaler,
		},
		vcsStatusUpdater:  vcsStatusUpdater,
		rootConfigBuilder: rootConfigBuilder,
		errorHandler:      errorHandler,
	}
}

type NeptuneWorkerProxy struct {
	logger             logging.Logger
	deploySignaler     deploySignaler
	commentCreator     commentCreator
	requirementChecker requirementChecker
	allocator          feature.Allocator
	prSignaler         prSignaler
}

func (p *NeptuneWorkerProxy) Handle(ctx context.Context, event Comment, cmd *command.Comment, roots []*valid.MergedProjectCfg, request *http.BufferedRequest) error {
	if cmd.Name == command.Apply {
		return p.handleForceApplies(ctx, event, cmd, roots)
	}
	// TODO: remove when we begin in-depth testing and rollout of pr mode
	// feature allocator is only temporary while we continue building out implementation
	shouldAllocate, err := p.allocator.ShouldAllocate(feature.LegacyDeprecation, feature.FeatureContext{
		RepoName: event.Pull.HeadRepo.FullName,
	})
	if err != nil {
		p.logger.ErrorContext(ctx, "unable to allocate pr mode")
		return nil
	}
	if !shouldAllocate {
		p.logger.InfoContext(ctx, "handler not configured for allocation")
		return nil
	}
	prRequest := pr.Request{
		Number:            event.Pull.Num,
		Revision:          event.Pull.HeadCommit,
		Repo:              event.Pull.HeadRepo,
		InstallationToken: event.InstallationToken,
		Branch:            event.Pull.HeadBranch,
		ValidateEnvs:      buildValidateEnvsFromComment(event),
	}
	run, err := p.prSignaler.SignalWithStartWorkflow(ctx, roots, prRequest)
	if err != nil {
		return errors.Wrap(err, "signaling workflow")
	}
	p.logger.InfoContext(ctx, "Signaled workflow.", map[string]interface{}{
		"workflow-id": run.GetID(), "run-id": run.GetRunID(),
	})
	return nil
}

func (p *NeptuneWorkerProxy) handleForceApplies(ctx context.Context, event Comment, cmd *command.Comment, roots []*valid.MergedProjectCfg) error {
	triggerInfo := workflows.DeployTriggerInfo{
		Type:  workflows.ManualTrigger,
		Force: cmd.ForceApply,
	}

	if cmd.IsForSpecificProject() {
		roots = partitionRootsByProject(cmd.ProjectName, roots)
	}

	if len(roots) == 0 {
		p.logger.WarnContext(ctx, "no roots detected")
		return nil
	}

	if err := p.requirementChecker.Check(ctx, requirement.Criteria{
		Repo:              event.BaseRepo,
		Branch:            event.Pull.HeadBranch,
		User:              event.User,
		InstallationToken: event.InstallationToken,
		TriggerInfo:       triggerInfo,
		OptionalPull:      &event.Pull,
		Roots:             roots,
	}); err != nil {
		return errors.Wrap(err, "checking deploy requirements")
	}

	if cmd.ForceApply {
		if err := p.commentCreator.CreateComment(event.BaseRepo, event.PullNum, warningMessage, ""); err != nil {
			p.logger.ErrorContext(ctx, err.Error())
		}
	}

	opts := deploy.RootDeployOptions{
		Repo:              event.BaseRepo,
		Branch:            event.Pull.HeadBranch,
		Revision:          event.Pull.HeadCommit,
		OptionalPullNum:   event.Pull.Num,
		Sender:            event.User,
		InstallationToken: event.InstallationToken,
		TriggerInfo:       triggerInfo,
	}

	for _, r := range roots {
		_, err := p.deploySignaler.SignalWithStartWorkflow(ctx, r, opts)
		if err != nil {
			return errors.Wrap(err, "signalling workflow")
		}
	}
	return nil
}

type CommentEventWorkerProxy struct {
	logger             logging.Logger
	scheduler          scheduler
	vcsStatusUpdater   statusUpdater
	rootConfigBuilder  rootConfigBuilder
	legacyHandler      *LegacyCommentHandler
	neptuneWorkerProxy *NeptuneWorkerProxy
	errorHandler       errorHandler
}

func (p *CommentEventWorkerProxy) Handle(ctx context.Context, request *http.BufferedRequest, event Comment, cmd *command.Comment) error {
	executor := p.errorHandler.WrapWithHandling(ctx, event, cmd.CommandName().String(), func(ctx context.Context) error {
		return p.handle(ctx, request, event, cmd)
	})
	return errors.Wrap(p.scheduler.Schedule(ctx, executor), "scheduling handle")
}

func (p *CommentEventWorkerProxy) handle(ctx context.Context, request *http.BufferedRequest, event Comment, cmd *command.Comment) error {
	roots, err := p.rootConfigBuilder.Build(ctx, &config.RepoCommit{
		Repo:          event.BaseRepo,
		Branch:        event.Pull.HeadBranch,
		Sha:           event.Pull.HeadCommit,
		OptionalPRNum: event.PullNum,
	}, event.InstallationToken)

	if err != nil {
		if _, statusErr := p.vcsStatusUpdater.UpdateCombined(ctx, event.HeadRepo, event.Pull, models.FailedVCSStatus, cmd.Name, "", err.Error()); statusErr != nil {
			p.logger.WarnContext(ctx, fmt.Sprintf("unable to update commit status: %v", statusErr))
		}
		return errors.Wrap(err, "getting project commands")
	}

	if len(roots) == 0 {
		p.logger.WarnContext(ctx, "no roots to process in comment")
		p.markSuccessStatuses(ctx, event, cmd)
		return nil
	}

	fxns := []func(ctx context.Context, event Comment, cmd *command.Comment, roots []*valid.MergedProjectCfg, request *http.BufferedRequest) error{
		p.legacyHandler.Handle,
		p.neptuneWorkerProxy.Handle,
	}
	var combinedErrors *multierror.Error
	for _, f := range fxns {
		err := p.scheduler.Schedule(ctx, func(ctx context.Context) error {
			return f(ctx, event, cmd, roots, request)
		})
		combinedErrors = multierror.Append(combinedErrors, err)
	}
	return combinedErrors.ErrorOrNil()
}

// TODO: do we need to keep marking plan as successful after legacy deprecation?
func (p *CommentEventWorkerProxy) markSuccessStatuses(ctx context.Context, event Comment, cmd *command.Comment) {
	if cmd.Name == command.Plan {
		for _, name := range []command.Name{command.Plan, command.PolicyCheck, command.Apply} {
			if _, statusErr := p.vcsStatusUpdater.UpdateCombined(ctx, event.HeadRepo, event.Pull, models.SuccessVCSStatus, name, "", "no modified roots"); statusErr != nil {
				p.logger.WarnContext(ctx, fmt.Sprintf("unable to update commit status: %v", statusErr))
			}
		}
	}

	if cmd.Name == command.Apply {
		if _, statusErr := p.vcsStatusUpdater.UpdateCombined(ctx, event.HeadRepo, event.Pull, models.SuccessVCSStatus, cmd.Name, "", "no modified roots"); statusErr != nil {
			p.logger.WarnContext(ctx, fmt.Sprintf("unable to update commit status: %v", statusErr))
		}
	}
}

func partitionRootsByProject(name string, cmds []*valid.MergedProjectCfg) []*valid.MergedProjectCfg {
	var cfgs []*valid.MergedProjectCfg
	for _, cmd := range cmds {
		if cmd.Name == name {
			cfgs = append(cfgs, cmd)
		}
	}
	return cfgs
}

func buildValidateEnvsFromComment(event Comment) []pr.ValidateEnvs {
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
