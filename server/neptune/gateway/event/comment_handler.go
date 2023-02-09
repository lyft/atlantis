package event

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/lyft/feature"
	"github.com/runatlantis/atlantis/server/neptune/template"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"github.com/runatlantis/atlantis/server/vcs/provider/github"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/http"
	"github.com/runatlantis/atlantis/server/logging"
)

const warningMessage = "⚠️ WARNING ⚠️\n\n You are force applying changes from your PR instead of merging into your default branch 🚀. This can have unpredictable consequences 🙏🏽 and should only be used in an emergency 🆘.\n\n To confirm behavior, review and confirm the plan within the generated atlantis/deploy GH check below.\n\n 𝐓𝐡𝐢𝐬 𝐚𝐜𝐭𝐢𝐨𝐧 𝐰𝐢𝐥𝐥 𝐛𝐞 𝐚𝐮𝐝𝐢𝐭𝐞𝐝.\n"
const PlatformModeApplyStatusMessage = "Bypassed for platform mode"

type LegacyApplyCommentInput struct{}

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
	snsWriter Writer,
	allocator feature.Allocator,
	scheduler scheduler,
	rootDeployer rootDeployer,
	rootConfigBuilder rootConfigBuilder,
	templateLoader template.Loader[any],
	vcsStatusUpdater events.VCSStatusUpdater,
	vcsClient vcs.Client) *CommentEventWorkerProxy {
	return &CommentEventWorkerProxy{
		logger:            logger,
		snsWriter:         snsWriter,
		allocator:         allocator,
		scheduler:         scheduler,
		vcsClient:         vcsClient,
		rootDeployer:      rootDeployer,
		rootConfigBuilder: rootConfigBuilder,
		vcsStatusUpdater:  vcsStatusUpdater,

		// cast the generic loader to be used with the type we need here
		templateLoader: template.Loader[LegacyApplyCommentInput](templateLoader),
	}
}

type CommentEventWorkerProxy struct {
	logger            logging.Logger
	snsWriter         Writer
	allocator         feature.Allocator
	scheduler         scheduler
	vcsClient         vcs.Client
	rootDeployer      rootDeployer
	rootConfigBuilder rootConfigBuilder
	vcsStatusUpdater  events.VCSStatusUpdater
	templateLoader    template.Loader[LegacyApplyCommentInput]
}

func (p *CommentEventWorkerProxy) Handle(ctx context.Context, request *http.BufferedRequest, event Comment, cmd *command.Comment) error {
	shouldAllocate, err := p.allocator.ShouldAllocate(feature.PlatformMode, feature.FeatureContext{
		RepoName: event.BaseRepo.FullName,
	})

	if err != nil {
		p.logger.ErrorContext(ctx, "unable to allocate platform mode")
		return p.forwardToSns(ctx, request)
	}

	// we use the feature flag as a kill switch without granular info on which repos platform mode is enabled on
	// so if feature flag is turned on and it's an apply command, we build the root configs to decide if platform mode is enabled for this repo
	if shouldAllocate && cmd.Name == command.Apply {

		// build root configs
		rootCfgs, err := p.rootConfigBuilder.Build(ctx, event.BaseRepo, event.Pull.HeadBranch, event.Pull.HeadCommit, event.InstallationToken, BuilderOptions{
			RepoFetcherOptions: github.RepoFetcherOptions{
				ShallowClone: true,
			},
			FileFetcherOptions: github.FileFetcherOptions{
				Sha:   event.Pull.HeadCommit,
				PRNum: event.PullNum,
			},
		})
		if err != nil {
			return errors.Wrap(err, "generating roots")
		}

		// if platform mode is not enabled for this repo, forward the request to the worker
		// manifest config for platform mode is by by repo, so if workflow mode is enabled for one root, it's enabled for all roots
		if rootCfgs[0].WorkflowMode != valid.PlatformWorkflowMode {
			return p.forwardToSns(ctx, request)
		}

		// Platform mode has been enabled for this repo
		if cmd.ForceApply {
			p.logger.InfoContext(ctx, "running force apply command")
			if err := p.vcsClient.CreateComment(event.BaseRepo, event.PullNum, warningMessage, ""); err != nil {
				p.logger.ErrorContext(ctx, err.Error())
			}
			return p.scheduler.Schedule(ctx, func(ctx context.Context) error {
				return p.forceApply(ctx, event)
			})
		}

		// atlantis/apply checkrun should not be in a failed state in platform mode. However, if it is due to some failures, we set the apply checkrun to successful here
		if _, statusErr := p.vcsStatusUpdater.UpdateCombined(ctx, event.HeadRepo, event.Pull, models.SuccessVCSStatus, command.Apply, "", PlatformModeApplyStatusMessage); statusErr != nil {
			p.logger.ErrorContext(ctx, errors.Wrap(statusErr, "updating atlantis apply to success").Error())
		}

		p.handleLegacyApplyCommand(ctx, event, cmd)
		return nil
	}
	return p.forwardToSns(ctx, request)
}

func (p *CommentEventWorkerProxy) handleLegacyApplyCommand(ctx context.Context, event Comment, cmd *command.Comment) {
	p.logger.InfoContext(ctx, "running legacy apply command on platform mode")

	// return error if loading template fails since we should have default templates configured
	comment, err := p.templateLoader.Load(template.LegacyApplyComment, event.BaseRepo, LegacyApplyCommentInput{})
	if err != nil {
		p.logger.ErrorContext(ctx, fmt.Sprintf("loading template: %s", template.LegacyApplyComment))
	}

	if err := p.vcsClient.CreateComment(event.BaseRepo, event.PullNum, comment, ""); err != nil {
		p.logger.ErrorContext(ctx, err.Error())
	}
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

func (p *CommentEventWorkerProxy) forceApply(ctx context.Context, event Comment) error {
	// TODO: consider supporting shallow cloning for comment based events too
	builderOptions := BuilderOptions{
		FileFetcherOptions: github.FileFetcherOptions{
			PRNum: event.PullNum,
		},
	}
	rootDeployOptions := RootDeployOptions{
		Repo:              event.HeadRepo,
		Branch:            event.Pull.HeadBranch,
		Revision:          event.Pull.HeadCommit,
		Sender:            event.User,
		InstallationToken: event.InstallationToken,
		BuilderOptions:    builderOptions,
		Trigger:           workflows.ManualTrigger,
	}
	return p.rootDeployer.Deploy(ctx, rootDeployOptions)
}
