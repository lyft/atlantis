package event

import (
	"bytes"
	"context"
	"time"

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

// templateLoader loads the template for a command
type templateLoader interface {
	Load(id template.Key, repo models.Repo, data any) (string, error)
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
	snsWriter Writer,
	allocator feature.Allocator,
	scheduler scheduler,
	rootDeployer rootDeployer,
	templateLoader template.Loader[any],
	vcsClient vcs.Client) *CommentEventWorkerProxy {
	return &CommentEventWorkerProxy{
		logger:         logger,
		snsWriter:      snsWriter,
		allocator:      allocator,
		scheduler:      scheduler,
		vcsClient:      vcsClient,
		rootDeployer:   rootDeployer,
		templateLoader: &templateLoader,
	}
}

type CommentEventWorkerProxy struct {
	logger         logging.Logger
	snsWriter      Writer
	allocator      feature.Allocator
	scheduler      scheduler
	vcsClient      vcs.Client
	rootDeployer   rootDeployer
	templateLoader templateLoader
}

func (p *CommentEventWorkerProxy) Handle(ctx context.Context, request *http.BufferedRequest, event Comment, cmd *command.Comment) error {
	shouldAllocate, err := p.allocator.ShouldAllocate(feature.PlatformMode, feature.FeatureContext{
		RepoName: event.BaseRepo.FullName,
	})

	if err != nil {
		p.logger.ErrorContext(ctx, "unable to allocate platform mode")
		return p.forwardToSns(ctx, request)
	}

	if shouldAllocate {
		if cmd.ForceApply {
			p.logger.InfoContext(ctx, "running force apply command")
			if err := p.vcsClient.CreateComment(event.BaseRepo, event.PullNum, warningMessage, ""); err != nil {
				p.logger.ErrorContext(ctx, err.Error())
			}
			return p.scheduler.Schedule(ctx, func(ctx context.Context) error {
				return p.forceApply(ctx, event)
			})
		}

		// notify user that apply command is deprecated on platform mode
		if cmd.Name == command.Apply {
			p.handleLegacyApplyCommand(ctx, event, cmd)
		}

	}
	return p.forwardToSns(ctx, request)
}

func (p *CommentEventWorkerProxy) handleLegacyApplyCommand(ctx context.Context, event Comment, cmd *command.Comment) {
	p.logger.InfoContext(ctx, "running legacy apply command on platform mode")

	// return error if loading template fails since we should have default templates configured
	comment, err := p.templateLoader.Load(template.LegacyApplyComment, event.BaseRepo, nil)
	if err != nil {
		p.logger.ErrorContext(ctx, "loading template", map[string]interface{}{
			"template": template.LegacyApplyComment,
		})
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
