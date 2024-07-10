package event

import (
	"context"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/models"
	contextUtils "github.com/runatlantis/atlantis/server/neptune/context"
	"github.com/runatlantis/atlantis/server/neptune/gateway/requirement"
	"github.com/runatlantis/atlantis/server/neptune/lyft/feature"
	"github.com/runatlantis/atlantis/server/neptune/sync"
	"github.com/runatlantis/atlantis/server/neptune/template"
	"github.com/runatlantis/atlantis/server/vcs/provider/github"
)

type PREvent interface {
	GetPullNum() int
	GetInstallationToken() int64
	GetRepo() models.Repo
}

func NewNeptuneErrorHandler(commentCreator *github.CommentCreator, cfg valid.GlobalCfg, logger logging.Logger, allocator feature.Allocator) *NeptunePREventErrorHandler {
	return &NeptunePREventErrorHandler{
		allocator: allocator,
		delegate: &PREventErrorHandler{
			commentCreator: commentCreator,
			templateLoader: &template.Loader[template.PRCommentData]{
				GlobalCfg: cfg,
			},
			logger: logger,
		},
	}
}

func NewLegacyErrorHandler(commentCreator *github.CommentCreator, cfg valid.GlobalCfg, logger logging.Logger, allocator feature.Allocator) *LegacyPREventErrorHandler {
	return &LegacyPREventErrorHandler{
		allocator: allocator,
		delegate: &PREventErrorHandler{
			commentCreator: commentCreator,
			templateLoader: &template.Loader[template.PRCommentData]{
				GlobalCfg: cfg,
			},
			logger: logger,
		},
	}
}

type LegacyPREventErrorHandler struct {
	allocator feature.Allocator
	delegate  *PREventErrorHandler
}

func (p *LegacyPREventErrorHandler) WrapWithHandling(ctx context.Context, event PREvent, commandName string, executor sync.Executor) sync.Executor {
	return executor
}

type NeptunePREventErrorHandler struct {
	allocator feature.Allocator
	delegate  *PREventErrorHandler
}

func (p *NeptunePREventErrorHandler) WrapWithHandling(ctx context.Context, event PREvent, commandName string, executor sync.Executor) sync.Executor {
	return p.delegate.WrapWithHandling(ctx, event, commandName, executor)
}

// PREventErrorHandler is used provide additional functionality for handlers that want to provide feedback to the user
// Currently this feedback is provided through a PR comment.
type PREventErrorHandler struct {
	commentCreator *github.CommentCreator
	templateLoader *template.Loader[template.PRCommentData]
	logger         logging.Logger
}

func (p *PREventErrorHandler) WrapWithHandling(ctx context.Context, event PREvent, commandName string, executor sync.Executor) sync.Executor {
	return func(ctx context.Context) error {
		if err := executor(ctx); err != nil {
			if e := p.handleErr(ctx, event, commandName, err); e != nil {
				p.logger.ErrorContext(context.WithValue(ctx, contextUtils.ErrKey, e), "handling error")
			}
			return err
		}
		return nil
	}
}

func (p *PREventErrorHandler) handleErr(ctx context.Context, event PREvent, commandName string, err error) error {
	body, e := p.loadTemplate(event, commandName, err)
	if e != nil {
		return errors.Wrap(e, "loading template")
	}
	e = p.commentCreator.CreateComment(ctx, event.GetInstallationToken(), event.GetRepo(), event.GetPullNum(), body)
	if e != nil {
		return errors.Wrap(e, "commenting on PR")
	}
	return nil
}

func (p *PREventErrorHandler) loadTemplate(event PREvent, commandName string, err error) (string, error) {
	data := template.PRCommentData{
		Command:      commandName,
		ErrorDetails: err.Error(),
	}

	var forbiddenError requirement.ForbiddenError

	if errors.As(err, &forbiddenError) {
		data.ForbiddenError = true
		data.ForbiddenErrorTemplate = forbiddenError.ErrorTemplate()
	} else {
		data.InternalError = true
	}

	return p.templateLoader.Load(template.PRComment, event.GetRepo(), data)
}
