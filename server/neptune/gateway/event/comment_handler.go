package event

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/lyft/feature"
	"github.com/runatlantis/atlantis/server/neptune/gateway/deploy"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"github.com/uber-go/tally/v4"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/http"
	"github.com/runatlantis/atlantis/server/logging"
)

const warningMessage = "⚠️ WARNING ⚠️\n\n You are force applying changes from your PR instead of merging into your default branch 🚀. This can have unpredictable consequences 🙏🏽 and should only be used in an emergency 🆘.\n\n To confirm behavior, review and confirm the plan within the generated atlantis/deploy GH check below.\n\n 𝐓𝐡𝐢𝐬 𝐚𝐜𝐭𝐢𝐨𝐧 𝐰𝐢𝐥𝐥 𝐛𝐞 𝐚𝐮𝐝𝐢𝐭𝐞𝐝.\n"

type LegacyApplyCommentInput struct{}

type statusUpdater interface {
	UpdateCombined(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.VCSStatus, cmdName fmt.Stringer, statusID string, output string) (string, error)
}

type commentCreator interface {
	CreateComment(repo models.Repo, pullNum int, comment string, command string) error
}

type projectCommandGetter interface {
	GetProjectCommands(cmdCtx *command.Context, baseRepo models.Repo, pull models.PullRequest) ([]command.ProjectContext, error)
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

func NewCommentEventWorkerProxy(
	logger logging.Logger,
	scope tally.Scope,
	snsWriter Writer,
	allocator feature.Allocator,
	scheduler scheduler,
	rootDeployer rootDeployer,
	commentCreator commentCreator,
	vcsStatusUpdater statusUpdater,
	globalCfg valid.GlobalCfg,
	projectCommandGetter projectCommandGetter,
) *CommentEventWorkerProxy {
	return &CommentEventWorkerProxy{
		logger:               logger,
		scope:                scope,
		snsWriter:            snsWriter,
		allocator:            allocator,
		scheduler:            scheduler,
		commentCreator:       commentCreator,
		rootDeployer:         rootDeployer,
		vcsStatusUpdater:     vcsStatusUpdater,
		globalCfg:            globalCfg,
		projectCommandGetter: projectCommandGetter,
	}
}

type CommentEventWorkerProxy struct {
	logger               logging.Logger
	scope                tally.Scope
	snsWriter            Writer
	allocator            feature.Allocator
	scheduler            scheduler
	commentCreator       commentCreator
	rootDeployer         rootDeployer
	vcsStatusUpdater     statusUpdater
	globalCfg            valid.GlobalCfg
	projectCommandGetter projectCommandGetter
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

	err = p.scheduler.Schedule(ctx, func(ctx context.Context) error {
		return p.handle(ctx, request, event, cmd)
	})

	return errors.Wrap(err, "scheduling handle")
}

func (p *CommentEventWorkerProxy) handle(ctx context.Context, request *http.BufferedRequest, event Comment, cmd *command.Comment) error {
	cmdCtx := &command.Context{
		HeadRepo:   event.HeadRepo,
		Pull:       event.Pull,
		Scope:      p.scope,
		User:       event.User,
		Log:        p.logger,
		Trigger:    command.CommentTrigger,
		RequestCtx: ctx,
	}

	roots, err := p.projectCommandGetter.GetProjectCommands(cmdCtx, event.BaseRepo, cmdCtx.Pull)
	if err != nil {
		if _, statusErr := p.vcsStatusUpdater.UpdateCombined(ctx, cmdCtx.HeadRepo, cmdCtx.Pull, models.FailedVCSStatus, cmd.Name, "", err.Error()); statusErr != nil {
			cmdCtx.Log.WarnContext(cmdCtx.RequestCtx, fmt.Sprintf("unable to update commit status: %v", statusErr))
		}
		return errors.Wrap(err, "getting project commands")
	}

	if len(roots) == 0 {
		p.logger.WarnContext(ctx, "no roots to process in comment")
		p.markSuccessStatuses(ctx, cmdCtx, cmd)
		return nil
	}

	platformModeRoots, defaultModeRoots := partitionRootsByMode(roots)
	p.notifyImpendingChanges(
		ctx,
		len(platformModeRoots) == len(roots),
		event,
		cmd,
	)

	if !cmd.ForceApply {
		return p.forwardToSns(ctx, request)
	}

	// first process any platform mode force applies.
	var rootNames []string
	for _, r := range platformModeRoots {
		rootNames = append(rootNames, r.ProjectName)
	}

	if err := p.forceApplyPlatformMode(ctx, event, rootNames); err != nil {
		return errors.Wrap(err, "force applying platform mode roots")
	}

	// if we have any legacy roots that need force applying we have to forward this request to our legacy worker
	// Note: this doesn't happen if we can't process our platform mode roots.
	if len(defaultModeRoots) > 0 {
		if err := p.forwardToSns(ctx, request); err != nil {
			return errors.Wrap(err, "forwarding force apply through sns")
		}
	}

	return nil
}

func (p *CommentEventWorkerProxy) markSuccessStatuses(ctx context.Context, cmdCtx *command.Context, cmd *command.Comment) {
	if cmd.Name == command.Plan {
		for _, name := range []command.Name{command.Plan, command.PolicyCheck, command.Apply} {
			if _, statusErr := p.vcsStatusUpdater.UpdateCombined(ctx, cmdCtx.HeadRepo, cmdCtx.Pull, models.SuccessVCSStatus, name, "", "no modified roots"); statusErr != nil {
				cmdCtx.Log.WarnContext(ctx, fmt.Sprintf("unable to update commit status: %v", statusErr))
			}
		}
	}

	if cmd.Name == command.Apply {
		if _, statusErr := p.vcsStatusUpdater.UpdateCombined(ctx, cmdCtx.HeadRepo, cmdCtx.Pull, models.SuccessVCSStatus, cmd.Name, "", "no modified roots"); statusErr != nil {
			cmdCtx.Log.WarnContext(cmdCtx.RequestCtx, fmt.Sprintf("unable to update commit status: %v", statusErr))
		}
	}
}

func (p *CommentEventWorkerProxy) notifyImpendingChanges(
	ctx context.Context, allPlatformMode bool, event Comment, cmd *command.Comment) {
	if !allPlatformMode {
		p.setQueuedStatus(ctx, event, cmd)
		return
	}

	// if we're fully on platform mode for the repo, all plans still get forwarded to legacy,
	// however, `atlantis apply` is not valid so we shouldn't set this to queued
	if cmd.Name != command.Apply {
		p.setQueuedStatus(ctx, event, cmd)
		return
	}

	// if all our roots are on platform mode and we're force applying, let's post a specific comment. Otherwise this happens on legacy worker
	// since the comment won't make sense in the partial case
	if cmd.ForceApply {
		if err := p.commentCreator.CreateComment(event.BaseRepo, event.PullNum, warningMessage, ""); err != nil {
			p.logger.ErrorContext(ctx, err.Error())
		}
	}
}

func partitionRootsByMode(cmds []command.ProjectContext) ([]command.ProjectContext, []command.ProjectContext) {
	var platformModeCmds []command.ProjectContext
	var defaultCmds []command.ProjectContext
	for _, cmd := range cmds {
		if cmd.WorkflowModeType == valid.PlatformWorkflowMode {
			platformModeCmds = append(platformModeCmds, cmd)
		} else {
			defaultCmds = append(defaultCmds, cmd)
		}
	}

	return platformModeCmds, defaultCmds
}

func (p *CommentEventWorkerProxy) setQueuedStatus(ctx context.Context, event Comment, cmd *command.Comment) {
	if p.shouldMarkEventQueued(event, cmd) {
		if _, err := p.vcsStatusUpdater.UpdateCombined(ctx, event.BaseRepo, event.Pull, models.QueuedVCSStatus, cmd.Name, "", "Request received. Adding to the queue..."); err != nil {
			p.logger.WarnContext(ctx, fmt.Sprintf("unable to update commit status: %s", err))
		}
	}
}

func (p *CommentEventWorkerProxy) handleLegacyComment(ctx context.Context, request *http.BufferedRequest, event Comment, cmd *command.Comment) error {
	p.setQueuedStatus(ctx, event, cmd)
	return p.forwardToSns(ctx, request)
}

func (p *CommentEventWorkerProxy) shouldMarkEventQueued(event Comment, cmd *command.Comment) bool {
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

func (p *CommentEventWorkerProxy) forwardToSns(ctx context.Context, request *http.BufferedRequest) error {
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

func (p *CommentEventWorkerProxy) forceApplyPlatformMode(ctx context.Context, event Comment, rootNames []string) error {
	rootDeployOptions := deploy.RootDeployOptions{
		RootNames:         rootNames,
		Repo:              event.HeadRepo,
		Branch:            event.Pull.HeadBranch,
		Revision:          event.Pull.HeadCommit,
		OptionalPullNum:   event.Pull.Num,
		Sender:            event.User,
		InstallationToken: event.InstallationToken,
		TriggerInfo: workflows.DeployTriggerInfo{
			Type:  workflows.ManualTrigger,
			Force: true,
		},
	}

	return p.rootDeployer.Deploy(ctx, rootDeployOptions)
}
