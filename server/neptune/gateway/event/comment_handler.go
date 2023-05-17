package event

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/lyft/feature"
	"github.com/runatlantis/atlantis/server/neptune/gateway/deploy"
	"github.com/runatlantis/atlantis/server/neptune/sync"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"github.com/uber-go/tally/v4"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/http"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/neptune/gateway/config"
	"github.com/runatlantis/atlantis/server/neptune/gateway/deploy/requirement"
)

const warningMessage = "⚠️ WARNING ⚠️\n\n You are force applying changes from your PR instead of merging into your default branch 🚀. This can have unpredictable consequences 🙏🏽 and should only be used in an emergency 🆘.\n\n To confirm behavior, review and confirm the plan within the generated atlantis/deploy GH check below.\n\n 𝐓𝐡𝐢𝐬 𝐚𝐜𝐭𝐢𝐨𝐧 𝐰𝐢𝐥𝐥 𝐛𝐞 𝐚𝐮𝐝𝐢𝐭𝐞𝐝.\n"

type LegacyApplyCommentInput struct{}

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

func NewCommentEventWorkerProxy(
	logger logging.Logger,
	scope tally.Scope,
	snsWriter Writer,
	allocator feature.Allocator,
	scheduler scheduler,
	signaler deploySignaler,
	commentCreator commentCreator,
	vcsStatusUpdater statusUpdater,
	globalCfg valid.GlobalCfg,
	rootConfigBuilder rootConfigBuilder,
	errorHandler errorHandler,
	requirementChecker requirementChecker,
) *CommentEventWorkerProxy {
	return &CommentEventWorkerProxy{
		logger:    logger,
		allocator: allocator,
		scheduler: scheduler,
		snsWorkerProxy: &SNSWorkerProxy{
			logger:           logger,
			vcsStatusUpdater: vcsStatusUpdater,
			snsWriter:        snsWriter,
			globalCfg:        globalCfg,
		},
		neptuneWorkerProxy: &NeptuneWorkerProxy{
			logger:             logger,
			signaler:           signaler,
			commentCreator:     commentCreator,
			requirementChecker: requirementChecker,
		},
		vcsStatusUpdater:  vcsStatusUpdater,
		rootConfigBuilder: rootConfigBuilder,
		errorHandler:      errorHandler,
	}
}

type NeptuneWorkerProxy struct {
	logger             logging.Logger
	signaler           deploySignaler
	commentCreator     commentCreator
	requirementChecker requirementChecker
}

func (p *NeptuneWorkerProxy) Handle(ctx context.Context, request *http.BufferedRequest, event Comment, cmd *command.Comment, roots []*valid.MergedProjectCfg) error {
	// currently the only comments on platform mode are applies, we can add to this as necessary.
	if cmd.Name != command.Apply {
		return nil
	}

	triggerInfo := workflows.DeployTriggerInfo{
		Type:  workflows.ManualTrigger,
		Force: cmd.ForceApply,
	}

	platformModeRoots := partitionRootsByMode(valid.PlatformWorkflowMode, roots)

	if cmd.IsForSpecificProject() {
		platformModeRoots = partitionRootsByProject(cmd.ProjectName, platformModeRoots)
	}

	if len(platformModeRoots) == 0 {
		p.logger.WarnContext(ctx, "no platform mode roots detected")
		return nil
	}

	if err := p.requirementChecker.Check(ctx, requirement.Criteria{
		Repo:              event.BaseRepo,
		Branch:            event.Pull.HeadBranch,
		User:              event.User,
		InstallationToken: event.InstallationToken,
		TriggerInfo:       triggerInfo,
		OptionalPull:      &event.Pull,
		Roots:             platformModeRoots,
	}); err != nil {
		return errors.Wrap(err, "checking deploy requirements")
	}

	// let's only post a force apply comment on the PR, if we are only operating on project OR if we are operating on all projects and
	// if we're fully on platform mode, otherwise there will be duplicates from the legacy worker and this.
	if cmd.ForceApply && (cmd.IsForSpecificProject() || len(platformModeRoots) == len(roots)) {
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

	for _, r := range platformModeRoots {
		_, err := p.signaler.SignalWithStartWorkflow(ctx, r, opts)
		if err != nil {
			return errors.Wrap(err, "signalling workflow")
		}
	}
	return nil
}

type SNSWorkerProxy struct {
	logger           logging.Logger
	vcsStatusUpdater statusUpdater
	snsWriter        Writer
	globalCfg        valid.GlobalCfg
}

func (p *SNSWorkerProxy) Handle(ctx context.Context, request *http.BufferedRequest, event Comment, cmd *command.Comment, roots []*valid.MergedProjectCfg) error {
	defaultModeRoots := partitionRootsByMode(valid.DefaultWorkflowMode, roots)

	// cut off force applies here itself, since the legacy worker doesn't check the root workflow mode type before attempting
	// a force apply
	if len(defaultModeRoots) == 0 && cmd.ForceApply {
		p.logger.InfoContext(ctx, "no default mode roots to force apply")
		return nil
	}

	// only set queued status if we have default mode roots, we don't currently need this in the temporal world
	// but this is subject to change
	if len(defaultModeRoots) > 0 {
		p.SetQueuedStatus(ctx, event, cmd)
	}

	// forward everything to sns for now since platform mode doesn't do anything w.r.t to comments atm.
	if err := p.ForwardToSns(ctx, request); err != nil {
		return errors.Wrap(err, "forwarding request through sns")
	}
	return nil
}

func (p *SNSWorkerProxy) ForwardToSns(ctx context.Context, request *http.BufferedRequest) error {
	buffer := bytes.NewBuffer([]byte{})
	if err := request.GetRequestWithContext(ctx).Write(buffer); err != nil {
		return errors.Wrap(err, "writing request to buffer")
	}

	if err := p.snsWriter.WriteWithContext(ctx, buffer.Bytes()); err != nil {
		return errors.Wrap(err, "writing buffer to sns")
	}
	p.logger.InfoContext(ctx, "proxied request to sns")

	return nil
}

func (p *SNSWorkerProxy) SetQueuedStatus(ctx context.Context, event Comment, cmd *command.Comment) {
	if p.shouldMarkEventQueued(event, cmd) {
		if _, err := p.vcsStatusUpdater.UpdateCombined(ctx, event.BaseRepo, event.Pull, models.QueuedVCSStatus, cmd.Name, "", "Request received. Adding to the queue..."); err != nil {
			p.logger.WarnContext(ctx, fmt.Sprintf("unable to update commit status: %s", err))
		}
	}
}

func (p *SNSWorkerProxy) shouldMarkEventQueued(event Comment, cmd *command.Comment) bool {
	// pending status should only be for plan and apply step
	if cmd.Name != command.Plan && cmd.Name != command.Apply {
		return false
	}
	// pull event should not be from a fork
	if event.Pull.HeadRepo.Owner != event.Pull.BaseRepo.Owner {
		return false
	}
	// pull event should not be from closed PR
	if event.Pull.State == models.ClosedPullState {
		return false
	}
	// pull event should not use an invalid base branch
	repo := p.globalCfg.MatchingRepo(event.Pull.BaseRepo.ID())
	return repo.BranchMatches(event.Pull.BaseBranch)
}

type CommentEventWorkerProxy struct {
	logger             logging.Logger
	allocator          feature.Allocator
	scheduler          scheduler
	vcsStatusUpdater   statusUpdater
	rootConfigBuilder  rootConfigBuilder
	snsWorkerProxy     *SNSWorkerProxy
	neptuneWorkerProxy *NeptuneWorkerProxy
	errorHandler       errorHandler
}

func (p *CommentEventWorkerProxy) Handle(ctx context.Context, request *http.BufferedRequest, event Comment, cmd *command.Comment) error {
	shouldAllocate, err := p.allocator.ShouldAllocate(feature.PlatformMode, feature.FeatureContext{
		RepoName: event.BaseRepo.FullName,
	})

	// typically we shouldn't be failing if we can't figure out the feature, however, there is some complex logic
	// that depends on us knowing which mode we are running on so in order to prevent unintended consequences, let's just
	// bail if this happens.
	if err != nil {
		p.logger.ErrorContext(ctx, "unable to allocate platform mode")
		return errors.Wrap(err, "unable to allocate platform mode feature")
	}

	if !shouldAllocate {
		return p.handleLegacyComment(ctx, request, event, cmd)
	}

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

	if err := p.snsWorkerProxy.Handle(ctx, request, event, cmd, roots); err != nil {
		return errors.Wrap(err, "handling event in legacy sns worker handler")
	}
	if err := p.neptuneWorkerProxy.Handle(ctx, request, event, cmd, roots); err != nil {
		return errors.Wrap(err, "handling event in signal temporal worker handler")
	}

	return nil
}

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

func partitionRootsByMode(mode valid.WorkflowModeType, cmds []*valid.MergedProjectCfg) []*valid.MergedProjectCfg {
	var cfgs []*valid.MergedProjectCfg
	for _, cmd := range cmds {
		if cmd.WorkflowMode == mode {
			cfgs = append(cfgs, cmd)
		}
	}

	return cfgs
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

func (p *CommentEventWorkerProxy) handleLegacyComment(ctx context.Context, request *http.BufferedRequest, event Comment, cmd *command.Comment) error {
	p.snsWorkerProxy.SetQueuedStatus(ctx, event, cmd)
	return p.snsWorkerProxy.ForwardToSns(ctx, request)
}
