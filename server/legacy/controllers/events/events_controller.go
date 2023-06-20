// Copyright 2017 HootSuite Media Inc.
//
// Licensed under the Apache License, Version 2.0 (the License);
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an AS IS BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// Modified hereafter by contributors to runatlantis/atlantis.

package events

import (
	"context"
	"fmt"
	"net/http"

	httputils "github.com/runatlantis/atlantis/server/legacy/http"

	requestErrors "github.com/runatlantis/atlantis/server/legacy/controllers/events/errors"
	"github.com/runatlantis/atlantis/server/legacy/controllers/events/handlers"
	"github.com/runatlantis/atlantis/server/legacy/events"
	"github.com/runatlantis/atlantis/server/legacy/events/vcs"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/models"
	key "github.com/runatlantis/atlantis/server/neptune/context"
	event_types "github.com/runatlantis/atlantis/server/neptune/gateway/event"
	github_converter "github.com/runatlantis/atlantis/server/vcs/provider/github/converter"
	github_request "github.com/runatlantis/atlantis/server/vcs/provider/github/request"
	"github.com/uber-go/tally/v4"
)

const (
	githubHeader = "X-Github-Event"
)

type commentEventHandler interface {
	Handle(ctx context.Context, request *httputils.BufferedRequest, event event_types.Comment) error
}

type prEventHandler interface {
	Handle(ctx context.Context, request *httputils.BufferedRequest, event event_types.PullRequest) error
}

type unsupportedPushEventHandler struct{}

func (h unsupportedPushEventHandler) Handle(ctx context.Context, event event_types.Push) error {
	return fmt.Errorf("push events are not supported in this context")
}

type unsupportedCheckRunEventHandler struct{}

func (h unsupportedCheckRunEventHandler) Handle(ctx context.Context, event event_types.CheckRun) error {
	return fmt.Errorf("check run events are not supported in this context")
}

type unsupportedCheckSuiteEventHandler struct{}

func (h unsupportedCheckSuiteEventHandler) Handle(ctx context.Context, event event_types.CheckSuite) error {
	return fmt.Errorf("check suite events are not supported in this context")
}

func NewRequestResolvers(
	providerResolverInitializer map[models.VCSHostType]func() RequestResolver,
	supportedProviders []models.VCSHostType,
) []RequestResolver {
	var resolvers []RequestResolver
	for provider, resolverInitializer := range providerResolverInitializer {
		for _, supportedProvider := range supportedProviders {
			if provider != supportedProvider {
				continue
			}

			resolvers = append(resolvers, resolverInitializer())
		}
	}

	return resolvers
}

func NewVCSEventsController(
	scope tally.Scope,
	githubWebhookSecret []byte,
	allowDraftPRs bool,
	commandRunner events.CommandRunner,
	commentParser events.CommentParsing,
	eventParser events.EventParsing,
	pullCleaner events.PullCleaner,
	repoAllowlistChecker *events.RepoAllowlistChecker,
	vcsClient vcs.Client,
	logger logging.Logger,
	applyDisabled bool,
	supportedVCSProviders []models.VCSHostType,
	repoConverter github_converter.RepoConverter,
	pullConverter github_converter.PullConverter,
	githubPullGetter github_converter.PullGetter,
	pullFetcher github_converter.PullFetcher,
) *VCSEventsController {
	prHandler := handlers.NewPullRequestEvent(
		repoAllowlistChecker, pullCleaner, logger, commandRunner,
	)

	commentHandler := handlers.NewCommentEvent(
		commentParser,
		repoAllowlistChecker,
		vcsClient,
		commandRunner,
		logger,
	)

	pullRequestReviewHandler := handlers.NewPullRequestReviewEvent(commandRunner, logger)

	// we don't support push events in the atlantis worker and these should never make it in the queue
	// in the first place, so if it happens, let's return an error and fail fast.
	pushHandler := unsupportedPushEventHandler{}

	// lazy map of resolver providers to their resolver
	// laziness ensures we only instantiate the providers we support.
	providerResolverInitializer := map[models.VCSHostType]func() RequestResolver{
		models.Github: func() RequestResolver {
			return github_request.NewHandler(
				logger,
				scope,
				githubWebhookSecret,
				pullFetcher,
				commentHandler,
				prHandler,
				pushHandler,
				pullRequestReviewHandler,
				unsupportedCheckRunEventHandler{},
				unsupportedCheckSuiteEventHandler{},
				allowDraftPRs,
				repoConverter,
				pullConverter,
				githubPullGetter,
			)
		},
	}

	router := &RequestRouter{
		Resolvers: NewRequestResolvers(providerResolverInitializer, supportedVCSProviders),
		Logger:    logger,
	}

	return &VCSEventsController{
		RequestRouter:        router,
		Logger:               logger,
		Scope:                scope,
		Parser:               eventParser,
		CommentParser:        commentParser,
		PREventHandler:       prHandler,
		CommentEventHandler:  commentHandler,
		ApplyDisabled:        applyDisabled,
		RepoAllowlistChecker: repoAllowlistChecker,
		SupportedVCSHosts:    supportedVCSProviders,
		VCSClient:            vcsClient,
	}
}

type RequestHandler interface {
	Handle(request *httputils.BufferedRequest) error
}

type RequestMatcher interface {
	Matches(request *httputils.BufferedRequest) bool
}

type RequestResolver interface {
	RequestHandler
	RequestMatcher
}

// TODO: once VCSEventsController is fully broken down this implementation can just live in there.
type RequestRouter struct {
	Resolvers []RequestResolver
	Logger    logging.Logger
}

func (p *RequestRouter) Route(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// we do this to allow for multiple reads to the request body
	request, err := httputils.NewBufferedRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		p.logAndWriteBody(ctx, w, err.Error(), map[string]interface{}{key.ErrKey.String(): err})
		return
	}

	for _, resolver := range p.Resolvers {
		if !resolver.Matches(request) {
			continue
		}

		err := resolver.Handle(request)

		if e, ok := err.(*requestErrors.RequestValidationError); ok {
			w.WriteHeader(http.StatusForbidden)
			p.logAndWriteBody(ctx, w, e.Error(), map[string]interface{}{key.ErrKey.String(): e})
			return
		}

		if e, ok := err.(*requestErrors.WebhookParsingError); ok {
			w.WriteHeader(http.StatusBadRequest)
			p.logAndWriteBody(ctx, w, e.Error(), map[string]interface{}{key.ErrKey.String(): e})
			return
		}

		if e, ok := err.(*requestErrors.EventParsingError); ok {
			w.WriteHeader(http.StatusBadRequest)
			p.logAndWriteBody(ctx, w, e.Error(), map[string]interface{}{key.ErrKey.String(): e})
			return
		}

		if e, ok := err.(*requestErrors.UnsupportedEventTypeError); ok {
			// historically we've just ignored these so for now let's just do that.
			w.WriteHeader(http.StatusOK)
			p.logAndWriteBody(ctx, w, e.Error(), map[string]interface{}{key.ErrKey.String(): e})
			return
		}

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			p.logAndWriteBody(ctx, w, err.Error(), map[string]interface{}{key.ErrKey.String(): err})
			return
		}

		w.WriteHeader(http.StatusOK)
		p.logAndWriteBody(ctx, w, "Processing...")
		return
	}

	w.WriteHeader(http.StatusInternalServerError)
	p.logAndWriteBody(ctx, w, "no resolver configured for request")
}

func (p *RequestRouter) logAndWriteBody(ctx context.Context, w http.ResponseWriter, msg string, fields ...map[string]interface{}) {
	fmt.Fprintln(w, msg)
	p.Logger.InfoContext(ctx, msg, fields...)
}

// VCSEventsController handles all webhook requests which signify 'events' in the
// VCS host, ex. GitHub.
// TODO: migrate all provider specific request handling into packaged resolver similar to github
type VCSEventsController struct {
	Logger               logging.Logger
	Scope                tally.Scope
	CommentParser        events.CommentParsing
	Parser               events.EventParsing
	PREventHandler       prEventHandler
	CommentEventHandler  commentEventHandler
	RequestRouter        *RequestRouter
	ApplyDisabled        bool
	RepoAllowlistChecker *events.RepoAllowlistChecker
	// SupportedVCSHosts is which VCS hosts Atlantis was configured upon
	// startup to support.
	SupportedVCSHosts []models.VCSHostType
	VCSClient         vcs.Client
}

// Post handles POST webhook requests.
func (e *VCSEventsController) Post(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(githubHeader) != "" {
		e.RequestRouter.Route(w, r)
		return
	}
	e.respond(w, logging.Debug, http.StatusBadRequest, "Ignoring request")
}

func (e *VCSEventsController) respond(w http.ResponseWriter, lvl logging.LogLevel, code int, format string, args ...interface{}) {
	response := fmt.Sprintf(format, args...)
	switch lvl {
	case logging.Error:
		e.Logger.Error(response)
	case logging.Info:
		e.Logger.Info(response)
	case logging.Warn:
		e.Logger.Warn(response)
	case logging.Debug:
		e.Logger.Debug(response)
	default:
		e.Logger.Error(response)
	}
	w.WriteHeader(code)
	fmt.Fprintln(w, response)
}
