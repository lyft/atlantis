package request

import (
	"context"
	"fmt"

	"github.com/palantir/go-githubapp/githubapp"
	"github.com/runatlantis/atlantis/server/http"

	"github.com/runatlantis/atlantis/server/controllers/events/handlers"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event"

	"github.com/google/go-github/v45/github"
	"github.com/runatlantis/atlantis/server/controllers/events/errors"
	"github.com/runatlantis/atlantis/server/events/metrics"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/vcs/provider/github/converter"
	"github.com/uber-go/tally/v4"
)

const (
	githubHeader    = "X-Github-Event"
	requestIDHeader = "X-Github-Delivery"
)



// interfaces used in Handler

// event handler interfaces
type commentEventHandler interface {
	Handle(ctx context.Context, request *http.BufferedRequest, e event.Comment) error
}

type prEventHandler interface {
	Handle(ctx context.Context, request *http.BufferedRequest, e event.PullRequest) error
}

type pushEventHandler interface {
	Handle(ctx context.Context, e event.Push) error
}

// converter interfaces
type pullEventConverter interface {
	Convert(event *github.PullRequestEvent) (event.PullRequest, error)
}

type commentEventConverter interface {
	Convert(event *github.IssueCommentEvent) (event.Comment, error)
}

type pushEventConverter interface {
	Convert(event *github.PushEvent) (event.Push, error)
}

// Matcher matches a provided request against some condition
type Matcher struct{}

func (h *Matcher) Matches(request *http.BufferedRequest) bool {
	return request.GetHeader(githubHeader) != ""
}

func NewHandler(
	logger logging.Logger,
	scope tally.Scope,
	webhookSecret []byte,
	commentHandler *handlers.CommentEvent,
	prHandler *handlers.PullRequestEvent,
	pushHandler pushEventHandler,
	allowDraftPRs bool,
	repoConverter converter.RepoConverter,
	pullConverter converter.PullConverter,
	pullGetter converter.PullGetter,
) *Handler {
	return &Handler{
		Matcher:        Matcher{},
		validator:      validator{},
		commentHandler: commentHandler,
		prHandler:      prHandler,
		parser:         &parser{},
		pullEventConverter: converter.PullEventConverter{
			PullConverter: pullConverter,
			AllowDraftPRs: allowDraftPRs,
		},
		commentEventConverter: converter.CommentEventConverter{
			PullConverter: converter.PullConverter{
				RepoConverter: repoConverter,
			},
			PullGetter: pullGetter,
		},
		pushEventConverter: converter.PushEvent{
			RepoConverter: repoConverter,
		},
		pushHandler:   pushHandler,
		webhookSecret: webhookSecret,
		logger:        logger,
		scope:         scope,
	}
}

type Handler struct {
	validator             requestValidator
	commentHandler        commentEventHandler
	prHandler             prEventHandler
	pushHandler           pushEventHandler
	parser                webhookParser
	pullEventConverter    pullEventConverter
	commentEventConverter commentEventConverter
	pushEventConverter    pushEventConverter
	webhookSecret         []byte
	logger                logging.Logger
	scope                 tally.Scope

	Matcher
}

func (h *Handler) Handle(r *http.BufferedRequest) error {
	// Validate the request against the optional webhook secret.
	payload, err := h.validator.Validate(r, h.webhookSecret)
	if err != nil {
		return &errors.RequestValidationError{Err: err}
	}

	ctx := context.WithValue(
		r.GetRequest().Context(),
		logging.RequestIDKey,
		r.GetHeader(requestIDHeader),
	)

	scope := h.scope.SubScope("github.event")

	event, err := h.parser.Parse(r, payload)
	if err != nil {
		return &errors.WebhookParsingError{Err: err}
	}

	// all github app events implement this interface
	installationSource, ok := event.(githubapp.InstallationSource)

	if !ok {
		return fmt.Errorf("unable to get installation id from request %s", r.GetHeader(requestIDHeader))
	}

	installationID := githubapp.GetInstallationIDFromEvent(installationSource)

	// this will be used to create the relevant installation client
	ctx = context.WithValue(ctx, logging.InstallationIDKey, installationID)

	switch event := event.(type) {
	case *github.IssueCommentEvent:
		err = h.handleCommentEvent(ctx, event, r)
		scope = scope.SubScope(fmt.Sprintf("comment.%s", *event.Action))
	case *github.PullRequestEvent:
		err = h.handlePullRequestEvent(ctx, event, r)
		scope = scope.SubScope(fmt.Sprintf("pr.%s", *event.Action))
	case *github.PushEvent:
		err = h.handlePushEvent(ctx, event)
		scope = scope.SubScope("push")
	default:
		h.logger.WarnContext(ctx, "Ignoring unsupported event")
	}

	if err != nil {
		scope.Counter(metrics.ExecutionErrorMetric).Inc(1)
		return err
	}

	scope.Counter(metrics.ExecutionSuccessMetric).Inc(1)
	return nil
}

func (h *Handler) handleCommentEvent(ctx context.Context, e *github.IssueCommentEvent, request *http.BufferedRequest) error {
	if e.GetAction() != "created" {
		h.logger.WarnContext(ctx, "Ignoring comment event since action was not created")
		return nil
	}

	commentEvent, err := h.commentEventConverter.Convert(e)
	if err != nil {
		return &errors.EventParsingError{Err: err}
	}
	ctx = context.WithValue(ctx, logging.RepositoryKey, commentEvent.BaseRepo.FullName)
	ctx = context.WithValue(ctx, logging.PullNumKey, commentEvent.PullNum)
	ctx = context.WithValue(ctx, logging.SHAKey, commentEvent.Pull.HeadCommit)

	return h.commentHandler.Handle(ctx, request, commentEvent)
}

func (h *Handler) handlePullRequestEvent(ctx context.Context, e *github.PullRequestEvent, request *http.BufferedRequest) error {
	pullEvent, err := h.pullEventConverter.Convert(e)

	if err != nil {
		return &errors.EventParsingError{Err: err}
	}
	ctx = context.WithValue(ctx, logging.RepositoryKey, pullEvent.Pull.BaseRepo.FullName)
	ctx = context.WithValue(ctx, logging.PullNumKey, pullEvent.Pull.Num)
	ctx = context.WithValue(ctx, logging.SHAKey, pullEvent.Pull.HeadCommit)

	return h.prHandler.Handle(ctx, request, pullEvent)
}

func (h *Handler) handlePushEvent(ctx context.Context, e *github.PushEvent) error {
	pushEvent, err := h.pushEventConverter.Convert(e)

	if err != nil {
		return &errors.EventParsingError{Err: err}
	}
	ctx = context.WithValue(ctx, logging.RepositoryKey, pushEvent.Repo.FullName)
	ctx = context.WithValue(ctx, logging.SHAKey, pushEvent.Sha)

	return h.pushHandler.Handle(ctx, pushEvent)
}
