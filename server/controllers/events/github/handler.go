package github

import (
	"context"
	"fmt"
	"time"

	"github.com/runatlantis/atlantis/server/http"

	"github.com/runatlantis/atlantis/server/controllers/events/handlers"

	"github.com/google/go-github/v31/github"
	"github.com/runatlantis/atlantis/server/controllers/events/errors"
	"github.com/runatlantis/atlantis/server/events/metrics"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"
	converters "github.com/runatlantis/atlantis/server/converters/github"
	"github.com/uber-go/tally"
)

const githubHeader = "X-Github-Event"

func NewRequestHandler(
	logger logging.SimpleLogging,
	scope tally.Scope,
	webhookSecret []byte,
	commentHandler *handlers.CommentEvent,
	prHandler *handlers.PullRequestEvent,
	allowDraftPRs bool,
	repoConverter converters.RepoConverter,
	pullConverter converters.PullConverter,
) *RequestHandler {
	return &RequestHandler{
		RequestMatcher: RequestMatcher{},
		validator:      RequestValidator{},
		commentHandler: commentHandler,
		prHandler:      prHandler,
		parser: &eventParser{
			allowDraftPRs: allowDraftPRs,
			repoConverter: repoConverter,
			pullConverter: pullConverter,
		},
		webhookSecret: webhookSecret,
		logger:        logger,
		scope:         scope,
	}
}

type RequestMatcher struct{}

func (h *RequestMatcher) Matches(request *http.CloneableRequest) bool {
	return request.GetHeader(githubHeader) != ""
}

type RequestHandler struct {
	validator      RequestValidator
	commentHandler *handlers.CommentEvent
	prHandler      *handlers.PullRequestEvent
	parser         *eventParser
	webhookSecret  []byte
	logger         logging.SimpleLogging
	scope          tally.Scope

	RequestMatcher
}

func (h *RequestHandler) Handle(r *http.CloneableRequest) error {
	// Validate the request against the optional webhook secret.
	payload, err := h.validator.Validate(r, h.webhookSecret)
	if err != nil {
		return &errors.RequestValidationError{Err: err}
	}

	githubReqID := r.GetHeader("X-Github-Delivery")
	logger := h.logger.With("gh-request-id", githubReqID)
	scope := h.scope.SubScope("github.event")

	event, _ := github.ParseWebHook(github.WebHookType(r.GetRequest()), payload)

	switch event := event.(type) {
	case *github.IssueCommentEvent:
		err = h.handleGithubCommentEvent(event, logger, r)
		scope = scope.SubScope(fmt.Sprintf("comment.%s", *event.Action))
	case *github.PullRequestEvent:
		err = h.handleGithubPullRequestEvent(logger, event, r)
		scope = scope.SubScope(fmt.Sprintf("pr.%s", *event.Action))
	default:
		logger.Warnf("Ignoring unsupported event: %s", githubReqID)
	}

	if err != nil {
		scope.Counter(metrics.ExecutionErrorMetric).Inc(1)
		return err
	}

	scope.Counter(metrics.ExecutionSuccessMetric).Inc(1)
	return nil
}

func (h *RequestHandler) handleGithubCommentEvent(event *github.IssueCommentEvent, logger logging.SimpleLogging, request *http.CloneableRequest) error {
	if event.GetAction() != "created" {
		logger.Warnf("Ignoring comment event since action was not created")
		return nil
	}

	baseRepo, user, pullNum, err := h.parser.ParseGithubIssueCommentEvent(event)

	if err != nil {
		return &errors.EventParsingError{Err: err}
	}

	// TODO: move this to parser
	eventTimestamp := time.Now()
	githubComment := event.Comment
	if githubComment != nil && githubComment.CreatedAt != nil {
		eventTimestamp = *githubComment.CreatedAt
	} else {
		h.scope.Counter("github_comment_missing_timestamp").Inc(1)
	}

	return h.commentHandler.Handle(context.TODO(), request, handlers.CommentEventInput{
		BaseRepo:      baseRepo,
		MaybeHeadRepo: nil,
		User:          user,
		MaybePull:     nil,
		PullNum:       pullNum,
		Comment:       event.Comment.GetBody(),
		VCSHost:       models.Github,
		Timestamp:     eventTimestamp,
	})
}

func (h *RequestHandler) handleGithubPullRequestEvent(logger logging.SimpleLogging, pullEvent *github.PullRequestEvent, request *http.CloneableRequest) error {
	pull, pullEventType, user, err := h.parser.ParseGithubPullEvent(pullEvent)
	if err != nil {
		return &errors.EventParsingError{Err: err}
	}
	logger.Debugf("identified event as type %q", pullEventType.String())

	// TODO: move this to parser
	eventTimestamp := time.Now()
	githubPullRequest := pullEvent.PullRequest
	if githubPullRequest != nil && githubPullRequest.UpdatedAt != nil {
		eventTimestamp = *githubPullRequest.UpdatedAt
	} else {
		h.scope.Counter("github_pr_missing_timestamp").Inc(1)
	}

	return h.prHandler.Handle(context.TODO(), request, handlers.PullRequestEventInput{
		Logger:    logger,
		Pull:      pull,
		User:      user,
		EventType: pullEventType,
		Timestamp: eventTimestamp,
	})
}
