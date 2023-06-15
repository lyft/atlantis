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
	"io"
	"net/http"
	"time"

	httputils "github.com/runatlantis/atlantis/server/legacy/http"

	"github.com/pkg/errors"
	requestErrors "github.com/runatlantis/atlantis/server/legacy/controllers/events/errors"
	"github.com/runatlantis/atlantis/server/legacy/controllers/events/handlers"
	"github.com/runatlantis/atlantis/server/legacy/events"
	"github.com/runatlantis/atlantis/server/legacy/events/vcs"
	"github.com/runatlantis/atlantis/server/legacy/events/vcs/bitbucketcloud"
	"github.com/runatlantis/atlantis/server/legacy/events/vcs/bitbucketserver"
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

// bitbucketEventTypeHeader is the same in both cloud and server.
const (
	bitbucketEventTypeHeader       = "X-Event-Key"
	bitbucketCloudRequestIDHeader  = "X-Request-UUID"
	bitbucketServerRequestIDHeader = "X-Request-ID"
	bitbucketServerSignatureHeader = "X-Hub-Signature"
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
	bitbucketWebhookSecret []byte,
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
		RequestRouter:          router,
		Logger:                 logger,
		Scope:                  scope,
		Parser:                 eventParser,
		CommentParser:          commentParser,
		PREventHandler:         prHandler,
		CommentEventHandler:    commentHandler,
		ApplyDisabled:          applyDisabled,
		RepoAllowlistChecker:   repoAllowlistChecker,
		SupportedVCSHosts:      supportedVCSProviders,
		VCSClient:              vcsClient,
		BitbucketWebhookSecret: bitbucketWebhookSecret,
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
	// BitbucketWebhookSecret is the secret added to this webhook via the Bitbucket
	// UI that identifies this call as coming from Bitbucket. If empty, no
	// request validation is done.
	BitbucketWebhookSecret []byte
}

// Post handles POST webhook requests.
func (e *VCSEventsController) Post(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(githubHeader) != "" {
		e.RequestRouter.Route(w, r)
		return
	} else if r.Header.Get(bitbucketEventTypeHeader) != "" {
		// Bitbucket Cloud and Server use the same event type header but they
		// use different request ID headers.
		if r.Header.Get(bitbucketCloudRequestIDHeader) != "" {
			if !e.supportsHost(models.BitbucketCloud) {
				e.respond(w, logging.Debug, http.StatusBadRequest, "Ignoring request since not configured to support Bitbucket Cloud")
				return
			}
			e.handleBitbucketCloudPost(w, r)
			return
		} else if r.Header.Get(bitbucketServerRequestIDHeader) != "" {
			if !e.supportsHost(models.BitbucketServer) {
				e.respond(w, logging.Debug, http.StatusBadRequest, "Ignoring request since not configured to support Bitbucket Server")
				return
			}
			e.handleBitbucketServerPost(w, r)
			return
		}
	}
	e.respond(w, logging.Debug, http.StatusBadRequest, "Ignoring request")
}

func (e *VCSEventsController) handleBitbucketCloudPost(w http.ResponseWriter, r *http.Request) {
	eventType := r.Header.Get(bitbucketEventTypeHeader)
	reqID := r.Header.Get(bitbucketCloudRequestIDHeader)
	defer r.Body.Close() // nolint: errcheck
	body, err := io.ReadAll(r.Body)
	if err != nil {
		e.respond(w, logging.Error, http.StatusBadRequest, "Unable to read body: %s %s=%s", err, bitbucketCloudRequestIDHeader, reqID)
		return
	}
	switch eventType {
	case bitbucketcloud.PullCreatedHeader, bitbucketcloud.PullUpdatedHeader, bitbucketcloud.PullFulfilledHeader, bitbucketcloud.PullRejectedHeader:
		e.handleBitbucketCloudPullRequestEvent(w, eventType, body, reqID, r)
		return
	case bitbucketcloud.PullCommentCreatedHeader:
		e.HandleBitbucketCloudCommentEvent(w, body, reqID, r)
		return
	default:
		e.respond(w, logging.Debug, http.StatusOK, "Ignoring unsupported event type %s %s=%s", eventType, bitbucketCloudRequestIDHeader, reqID)
	}
}

func (e *VCSEventsController) handleBitbucketServerPost(w http.ResponseWriter, r *http.Request) {
	eventType := r.Header.Get(bitbucketEventTypeHeader)
	reqID := r.Header.Get(bitbucketServerRequestIDHeader)
	sig := r.Header.Get(bitbucketServerSignatureHeader)
	defer r.Body.Close() // nolint: errcheck
	body, err := io.ReadAll(r.Body)
	if err != nil {
		e.respond(w, logging.Error, http.StatusBadRequest, "Unable to read body: %s %s=%s", err, bitbucketServerRequestIDHeader, reqID)
		return
	}
	if eventType == bitbucketserver.DiagnosticsPingHeader {
		// Specially handle the diagnostics:ping event because Bitbucket Server
		// doesn't send the signature with this event for some reason.
		e.respond(w, logging.Info, http.StatusOK, "Successfully received %s event %s=%s", eventType, bitbucketServerRequestIDHeader, reqID)
		return
	}
	if len(e.BitbucketWebhookSecret) > 0 {
		if err := bitbucketserver.ValidateSignature(body, sig, e.BitbucketWebhookSecret); err != nil {
			e.respond(w, logging.Warn, http.StatusBadRequest, errors.Wrap(err, "request did not pass validation").Error())
			return
		}
	}
	switch eventType {
	case bitbucketserver.PullCreatedHeader, bitbucketserver.PullMergedHeader, bitbucketserver.PullDeclinedHeader, bitbucketserver.PullDeletedHeader:
		e.handleBitbucketServerPullRequestEvent(w, eventType, body, reqID, r)
		return
	case bitbucketserver.PullCommentCreatedHeader:
		e.HandleBitbucketServerCommentEvent(w, body, reqID, r)
		return
	default:
		e.respond(w, logging.Debug, http.StatusOK, "Ignoring unsupported event type %s %s=%s", eventType, bitbucketServerRequestIDHeader, reqID)
	}
}

// HandleBitbucketCloudCommentEvent handles comment events from Bitbucket.
func (e *VCSEventsController) HandleBitbucketCloudCommentEvent(w http.ResponseWriter, body []byte, reqID string, request *http.Request) {
	pull, baseRepo, headRepo, user, comment, err := e.Parser.ParseBitbucketCloudPullCommentEvent(body)
	if err != nil {
		e.respond(w, logging.Error, http.StatusBadRequest, "Error parsing pull data: %s %s=%s", err, bitbucketCloudRequestIDHeader, reqID)
		return
	}
	eventTimestamp := time.Now()
	lvl := logging.Debug
	cloneableRequest, err := httputils.NewBufferedRequest(request)
	if err != nil {
		e.respond(w, lvl, http.StatusInternalServerError, err.Error())
		return
	}
	err = e.CommentEventHandler.Handle(context.TODO(), cloneableRequest, event_types.Comment{
		BaseRepo:  baseRepo,
		HeadRepo:  headRepo,
		Pull:      pull,
		User:      user,
		PullNum:   pull.Num,
		Comment:   comment,
		VCSHost:   models.BitbucketCloud,
		Timestamp: eventTimestamp,
	})

	if err != nil {
		e.respond(w, lvl, http.StatusInternalServerError, err.Error())
		return
	}

	e.respond(w, lvl, http.StatusOK, err.Error())
}

// HandleBitbucketServerCommentEvent handles comment events from Bitbucket.
func (e *VCSEventsController) HandleBitbucketServerCommentEvent(w http.ResponseWriter, body []byte, reqID string, request *http.Request) {
	pull, baseRepo, headRepo, user, comment, err := e.Parser.ParseBitbucketServerPullCommentEvent(body)
	if err != nil {
		e.respond(w, logging.Error, http.StatusBadRequest, "Error parsing pull data: %s %s=%s", err, bitbucketCloudRequestIDHeader, reqID)
		return
	}
	eventTimestamp := time.Now()
	lvl := logging.Debug
	cloneableRequest, err := httputils.NewBufferedRequest(request)
	if err != nil {
		e.respond(w, lvl, http.StatusInternalServerError, err.Error())
		return
	}
	err = e.CommentEventHandler.Handle(context.TODO(), cloneableRequest, event_types.Comment{
		BaseRepo:  baseRepo,
		HeadRepo:  headRepo,
		Pull:      pull,
		User:      user,
		PullNum:   pull.Num,
		Comment:   comment,
		VCSHost:   models.BitbucketServer,
		Timestamp: eventTimestamp,
	})

	if err != nil {
		e.respond(w, lvl, http.StatusInternalServerError, err.Error())
		return
	}

	e.respond(w, lvl, http.StatusOK, "")
}

func (e *VCSEventsController) handleBitbucketCloudPullRequestEvent(w http.ResponseWriter, eventType string, body []byte, reqID string, request *http.Request) {
	pull, _, _, user, err := e.Parser.ParseBitbucketCloudPullEvent(body)
	if err != nil {
		e.respond(w, logging.Error, http.StatusBadRequest, "Error parsing pull data: %s %s=%s", err, bitbucketCloudRequestIDHeader, reqID)
		return
	}
	pullEventType := e.Parser.GetBitbucketCloudPullEventType(eventType)
	e.Logger.Info(fmt.Sprintf("identified event as type %q", pullEventType.String()))
	eventTimestamp := time.Now()
	// TODO: move this to the outer most function similar to github
	lvl := logging.Debug

	cloneableRequest, err := httputils.NewBufferedRequest(request)
	if err != nil {
		e.respond(w, lvl, http.StatusInternalServerError, err.Error())
		return
	}
	err = e.PREventHandler.Handle(context.TODO(), cloneableRequest, event_types.PullRequest{
		Pull:      pull,
		User:      user,
		EventType: pullEventType,
		Timestamp: eventTimestamp,
	})

	if err != nil {
		e.respond(w, lvl, http.StatusInternalServerError, err.Error())
		return
	}
	e.respond(w, lvl, http.StatusOK, "")
}

func (e *VCSEventsController) handleBitbucketServerPullRequestEvent(w http.ResponseWriter, eventType string, body []byte, reqID string, request *http.Request) {
	pull, _, _, user, err := e.Parser.ParseBitbucketServerPullEvent(body)
	if err != nil {
		e.respond(w, logging.Error, http.StatusBadRequest, "Error parsing pull data: %s %s=%s", err, bitbucketServerRequestIDHeader, reqID)
		return
	}
	pullEventType := e.Parser.GetBitbucketServerPullEventType(eventType)
	e.Logger.Info(fmt.Sprintf("identified event as type %q", pullEventType.String()))
	eventTimestamp := time.Now()
	lvl := logging.Debug
	cloneableRequest, err := httputils.NewBufferedRequest(request)
	if err != nil {
		e.respond(w, lvl, http.StatusInternalServerError, err.Error())
		return
	}
	err = e.PREventHandler.Handle(context.TODO(), cloneableRequest, event_types.PullRequest{
		Pull:      pull,
		User:      user,
		EventType: pullEventType,
		Timestamp: eventTimestamp,
	})

	if err != nil {
		e.respond(w, lvl, http.StatusInternalServerError, err.Error())
		return
	}
	e.respond(w, lvl, http.StatusOK, "Processing...")
}

// supportsHost returns true if h is in e.SupportedVCSHosts and false otherwise.
func (e *VCSEventsController) supportsHost(h models.VCSHostType) bool {
	for _, supported := range e.SupportedVCSHosts {
		if h == supported {
			return true
		}
	}
	return false
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
