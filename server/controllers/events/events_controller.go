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
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v31/github"
	"github.com/mcdafydd/go-azuredevops/azuredevops"
	"github.com/microcosm-cc/bluemonday"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/events/vcs/bitbucketcloud"
	"github.com/runatlantis/atlantis/server/events/vcs/bitbucketserver"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/uber-go/tally"
	gitlab "github.com/xanzy/go-gitlab"
)

const githubHeader = "X-Github-Event"
const gitlabHeader = "X-Gitlab-Event"
const azuredevopsHeader = "Request-Id"

// bitbucketEventTypeHeader is the same in both cloud and server.
const bitbucketEventTypeHeader = "X-Event-Key"
const bitbucketCloudRequestIDHeader = "X-Request-UUID"
const bitbucketServerRequestIDHeader = "X-Request-ID"
const bitbucketServerSignatureHeader = "X-Hub-Signature"

// VCSEventsController handles all webhook requests which signify 'events' in the
// VCS host, ex. GitHub.
type VCSEventsController struct {
	CommandRunner events.CommandRunner
	PullCleaner   events.PullCleaner
	Logger        logging.SimpleLogging
	Scope         tally.Scope
	Parser        events.EventParsing
	CommentParser events.CommentParsing
	ApplyDisabled bool
	// GithubWebhookSecret is the secret added to this webhook via the GitHub
	// UI that identifies this call as coming from GitHub. If empty, no
	// request validation is done.
	GithubWebhookSecret          []byte
	GithubRequestValidator       GithubRequestValidator
	GitlabRequestParserValidator GitlabRequestParserValidator
	// GitlabWebhookSecret is the secret added to this webhook via the GitLab
	// UI that identifies this call as coming from GitLab. If empty, no
	// request validation is done.
	GitlabWebhookSecret  []byte
	RepoAllowlistChecker *events.RepoAllowlistChecker
	// SupportedVCSHosts is which VCS hosts Atlantis was configured upon
	// startup to support.
	SupportedVCSHosts []models.VCSHostType
	VCSClient         vcs.Client
	TestingMode       bool
	// BitbucketWebhookSecret is the secret added to this webhook via the Bitbucket
	// UI that identifies this call as coming from Bitbucket. If empty, no
	// request validation is done.
	BitbucketWebhookSecret []byte
	// AzureDevopsWebhookUser is the Basic authentication username added to this
	// webhook via the Azure DevOps UI that identifies this call as coming from your
	// Azure DevOps Team Project. If empty, no request validation is done.
	// For more information, see https://docs.microsoft.com/en-us/azure/devops/service-hooks/services/webhooks?view=azure-devops
	AzureDevopsWebhookBasicUser []byte
	// AzureDevopsWebhookPassword is the Basic authentication password added to this
	// webhook via the Azure DevOps UI that identifies this call as coming from your
	// Azure DevOps Team Project. If empty, no request validation is done.
	AzureDevopsWebhookBasicPassword []byte
	AzureDevopsRequestValidator     AzureDevopsRequestValidator
}

// Post handles POST webhook requests.
func (e *VCSEventsController) Post(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(githubHeader) != "" {
		if !e.supportsHost(models.Github) {
			e.respond(w, logging.Debug, http.StatusBadRequest, "Ignoring request since not configured to support GitHub")
			return
		}
		e.Logger.Debugf("handling GitHub post")
		e.handleGithubPost(w, r)
		return
	} else if r.Header.Get(gitlabHeader) != "" {
		if !e.supportsHost(models.Gitlab) {
			e.respond(w, logging.Debug, http.StatusBadRequest, "Ignoring request since not configured to support GitLab")
			return
		}
		e.Logger.Debugf("handling GitLab post")
		e.handleGitlabPost(w, r)
		return
	} else if r.Header.Get(bitbucketEventTypeHeader) != "" {
		// Bitbucket Cloud and Server use the same event type header but they
		// use different request ID headers.
		if r.Header.Get(bitbucketCloudRequestIDHeader) != "" {
			if !e.supportsHost(models.BitbucketCloud) {
				e.respond(w, logging.Debug, http.StatusBadRequest, "Ignoring request since not configured to support Bitbucket Cloud")
				return
			}
			e.Logger.Debugf("handling Bitbucket Cloud post")
			e.handleBitbucketCloudPost(w, r)
			return
		} else if r.Header.Get(bitbucketServerRequestIDHeader) != "" {
			if !e.supportsHost(models.BitbucketServer) {
				e.respond(w, logging.Debug, http.StatusBadRequest, "Ignoring request since not configured to support Bitbucket Server")
				return
			}
			e.Logger.Debugf("handling Bitbucket Server post")
			e.handleBitbucketServerPost(w, r)
			return
		}
	} else if r.Header.Get(azuredevopsHeader) != "" {
		if !e.supportsHost(models.AzureDevops) {
			e.respond(w, logging.Debug, http.StatusBadRequest, "Ignoring request since not configured to support AzureDevops")
			return
		}
		e.Logger.Debugf("handling AzureDevops post")
		e.handleAzureDevopsPost(w, r)
		return
	}
	e.respond(w, logging.Debug, http.StatusBadRequest, "Ignoring request")
}

type HttpError struct {
	err  error
	code int
}

type HttpResponse struct {
	body string
	err  HttpError
}

func (e *VCSEventsController) handleGithubPost(w http.ResponseWriter, r *http.Request) {
	// Validate the request against the optional webhook secret.
	payload, err := e.GithubRequestValidator.Validate(r, e.GithubWebhookSecret)
	if err != nil {
		e.respond(w, logging.Warn, http.StatusBadRequest, err.Error())
		return
	}

	githubReqID := r.Header.Get("X-Github-Delivery")
	logger := e.Logger.With("gh-request-id", githubReqID)
	scope := e.Scope.SubScope("github.event")

	logger.Debugf("request valid")

	event, _ := github.ParseWebHook(github.WebHookType(r), payload)

	var resp HttpResponse

	switch event := event.(type) {
	case *github.IssueCommentEvent:
		resp = e.HandleGithubCommentEvent(event, githubReqID, logger)
		scope = scope.SubScope(fmt.Sprintf("comment.%s", *event.Action))
	case *github.PullRequestEvent:
		resp = e.HandleGithubPullRequestEvent(logger, event, githubReqID)
		scope = scope.SubScope(fmt.Sprintf("pr.%s", *event.Action))
	default:
		resp = HttpResponse{
			body: fmt.Sprintf("Ignoring unsupported event %s", githubReqID),
		}
	}

	if resp.err.code != 0 {
		logger.Errorf("error handling gh post code: %d err: %s", resp.err.code, resp.err.err.Error())
		scope.Counter(fmt.Sprintf("error_%d", resp.err.code)).Inc(1)
		w.WriteHeader(resp.err.code)
		fmt.Fprintln(w, resp.err.err.Error())
		return
	}

	scope.Counter(fmt.Sprintf("success_%d", http.StatusOK)).Inc(1)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, resp.body)
}

func (e *VCSEventsController) handleBitbucketCloudPost(w http.ResponseWriter, r *http.Request) {
	eventType := r.Header.Get(bitbucketEventTypeHeader)
	reqID := r.Header.Get(bitbucketCloudRequestIDHeader)
	defer r.Body.Close() // nolint: errcheck
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		e.respond(w, logging.Error, http.StatusBadRequest, "Unable to read body: %s %s=%s", err, bitbucketCloudRequestIDHeader, reqID)
		return
	}
	switch eventType {
	case bitbucketcloud.PullCreatedHeader, bitbucketcloud.PullUpdatedHeader, bitbucketcloud.PullFulfilledHeader, bitbucketcloud.PullRejectedHeader:
		e.Logger.Debugf("handling as pull request state changed event")
		e.handleBitbucketCloudPullRequestEvent(w, eventType, body, reqID)
		return
	case bitbucketcloud.PullCommentCreatedHeader:
		e.Logger.Debugf("handling as comment created event")
		e.HandleBitbucketCloudCommentEvent(w, body, reqID)
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
	body, err := ioutil.ReadAll(r.Body)
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
		e.Logger.Debugf("handling as pull request state changed event")
		e.handleBitbucketServerPullRequestEvent(w, eventType, body, reqID)
		return
	case bitbucketserver.PullCommentCreatedHeader:
		e.Logger.Debugf("handling as comment created event")
		e.HandleBitbucketServerCommentEvent(w, body, reqID)
		return
	default:
		e.respond(w, logging.Debug, http.StatusOK, "Ignoring unsupported event type %s %s=%s", eventType, bitbucketServerRequestIDHeader, reqID)
	}
}

func (e *VCSEventsController) handleAzureDevopsPost(w http.ResponseWriter, r *http.Request) {
	// Validate the request against the optional basic auth username and password.
	payload, err := e.AzureDevopsRequestValidator.Validate(r, e.AzureDevopsWebhookBasicUser, e.AzureDevopsWebhookBasicPassword)
	if err != nil {
		e.respond(w, logging.Warn, http.StatusUnauthorized, err.Error())
		return
	}
	e.Logger.Debugf("request valid")

	azuredevopsReqID := "Request-Id=" + r.Header.Get("Request-Id")
	event, err := azuredevops.ParseWebHook(payload)
	if err != nil {
		e.respond(w, logging.Error, http.StatusBadRequest, "Failed parsing webhook: %v %s", err, azuredevopsReqID)
		return
	}
	switch event.PayloadType {
	case azuredevops.PullRequestCommentedEvent:
		e.Logger.Debugf("handling as pull request commented event")
		e.HandleAzureDevopsPullRequestCommentedEvent(w, event, azuredevopsReqID)
	case azuredevops.PullRequestEvent:
		e.Logger.Debugf("handling as pull request event")
		e.HandleAzureDevopsPullRequestEvent(w, event, azuredevopsReqID)
	default:
		e.respond(w, logging.Debug, http.StatusOK, "Ignoring unsupported event: %v %s", event.PayloadType, azuredevopsReqID)
	}
}

// HandleGithubCommentEvent handles comment events from GitHub where Atlantis
// commands can come from. It's exported to make testing easier.
func (e *VCSEventsController) HandleGithubCommentEvent(event *github.IssueCommentEvent, githubReqID string, logger logging.SimpleLogging) HttpResponse {
	if event.GetAction() != "created" {
		return HttpResponse{
			body: fmt.Sprintf("Ignoring comment event since action was not created %s", githubReqID),
		}
	}

	baseRepo, user, pullNum, err := e.Parser.ParseGithubIssueCommentEvent(event)

	wrapped := errors.Wrapf(err, "Failed parsing event: %s", githubReqID)
	if err != nil {
		return HttpResponse{
			body: wrapped.Error(),
			err: HttpError{
				code: http.StatusBadRequest,
				err:  wrapped,
			},
		}
	}
	eventTimestamp := time.Now()
	githubComment := event.Comment
	if githubComment != nil && githubComment.CreatedAt != nil {
		eventTimestamp = *githubComment.CreatedAt
	} else {
		e.Scope.Counter("github_comment_missing_timestamp").Inc(1)
	}
	// We pass in nil for maybeHeadRepo because the head repo data isn't
	// available in the GithubIssueComment event.
	return e.handleCommentEvent(logger, baseRepo, nil, nil, user, pullNum, event.Comment.GetBody(), models.Github, eventTimestamp)
}

// HandleBitbucketCloudCommentEvent handles comment events from Bitbucket.
func (e *VCSEventsController) HandleBitbucketCloudCommentEvent(w http.ResponseWriter, body []byte, reqID string) {
	pull, baseRepo, headRepo, user, comment, err := e.Parser.ParseBitbucketCloudPullCommentEvent(body)
	if err != nil {
		e.respond(w, logging.Error, http.StatusBadRequest, "Error parsing pull data: %s %s=%s", err, bitbucketCloudRequestIDHeader, reqID)
		return
	}
	eventTimestamp := time.Now()
	resp := e.handleCommentEvent(e.Logger, baseRepo, &headRepo, &pull, user, pull.Num, comment, models.BitbucketCloud, eventTimestamp)

	//TODO: move this to the outer most function similar to github
	lvl := logging.Debug
	code := http.StatusOK
	msg := resp.body
	if resp.err.code != 0 {
		lvl = logging.Error
		code = resp.err.code
		msg = resp.err.err.Error()
	}
	e.respond(w, lvl, code, msg)
}

// HandleBitbucketServerCommentEvent handles comment events from Bitbucket.
func (e *VCSEventsController) HandleBitbucketServerCommentEvent(w http.ResponseWriter, body []byte, reqID string) {
	pull, baseRepo, headRepo, user, comment, err := e.Parser.ParseBitbucketServerPullCommentEvent(body)
	if err != nil {
		e.respond(w, logging.Error, http.StatusBadRequest, "Error parsing pull data: %s %s=%s", err, bitbucketCloudRequestIDHeader, reqID)
		return
	}
	eventTimestamp := time.Now()
	resp := e.handleCommentEvent(e.Logger, baseRepo, &headRepo, &pull, user, pull.Num, comment, models.BitbucketCloud, eventTimestamp)

	//TODO: move this to the outer most function similar to github
	lvl := logging.Debug
	code := http.StatusOK
	msg := resp.body
	if resp.err.code != 0 {
		lvl = logging.Error
		code = resp.err.code
		msg = resp.err.err.Error()
	}
	e.respond(w, lvl, code, msg)
}

func (e *VCSEventsController) handleBitbucketCloudPullRequestEvent(w http.ResponseWriter, eventType string, body []byte, reqID string) {
	pull, baseRepo, headRepo, user, err := e.Parser.ParseBitbucketCloudPullEvent(body)
	if err != nil {
		e.respond(w, logging.Error, http.StatusBadRequest, "Error parsing pull data: %s %s=%s", err, bitbucketCloudRequestIDHeader, reqID)
		return
	}
	pullEventType := e.Parser.GetBitbucketCloudPullEventType(eventType)
	e.Logger.Infof("identified event as type %q", pullEventType.String())
	eventTimestamp := time.Now()
	resp := e.handlePullRequestEvent(e.Logger, baseRepo, headRepo, pull, user, pullEventType, eventTimestamp)

	//TODO: move this to the outer most function similar to github
	lvl := logging.Debug
	code := http.StatusOK
	msg := resp.body
	if resp.err.code != 0 {
		lvl = logging.Error
		code = resp.err.code
		msg = resp.err.err.Error()
	}
	e.respond(w, lvl, code, msg)
}

func (e *VCSEventsController) handleBitbucketServerPullRequestEvent(w http.ResponseWriter, eventType string, body []byte, reqID string) {
	pull, baseRepo, headRepo, user, err := e.Parser.ParseBitbucketServerPullEvent(body)
	if err != nil {
		e.respond(w, logging.Error, http.StatusBadRequest, "Error parsing pull data: %s %s=%s", err, bitbucketServerRequestIDHeader, reqID)
		return
	}
	pullEventType := e.Parser.GetBitbucketServerPullEventType(eventType)
	e.Logger.Infof("identified event as type %q", pullEventType.String())
	eventTimestamp := time.Now()
	resp := e.handlePullRequestEvent(e.Logger, baseRepo, headRepo, pull, user, pullEventType, eventTimestamp)

	//TODO: move this to the outer most function similar to github
	lvl := logging.Debug
	code := http.StatusOK
	msg := resp.body
	if resp.err.code != 0 {
		lvl = logging.Error
		code = resp.err.code
		msg = resp.err.err.Error()
	}
	e.respond(w, lvl, code, msg)
}

// HandleGithubPullRequestEvent will delete any locks associated with the pull
// request if the event is a pull request closed event. It's exported to make
// testing easier.
func (e *VCSEventsController) HandleGithubPullRequestEvent(logger logging.SimpleLogging, pullEvent *github.PullRequestEvent, githubReqID string) HttpResponse {
	pull, pullEventType, baseRepo, headRepo, user, err := e.Parser.ParseGithubPullEvent(pullEvent)
	if err != nil {
		wrapped := errors.Wrapf(err, "Error parsing pull data: %s %s", err, githubReqID)
		return HttpResponse{
			body: wrapped.Error(),
			err: HttpError{
				code: http.StatusBadRequest,
				err:  wrapped,
			},
		}
	}
	logger.Debugf("identified event as type %q", pullEventType.String())
	eventTimestamp := time.Now()
	githubPullRequest := pullEvent.PullRequest
	if githubPullRequest != nil && githubPullRequest.UpdatedAt != nil {
		eventTimestamp = *githubPullRequest.UpdatedAt
	} else {
		e.Scope.Counter("github_pr_missing_timestamp").Inc(1)
	}
	return e.handlePullRequestEvent(logger, baseRepo, headRepo, pull, user, pullEventType, eventTimestamp)
}

func (e *VCSEventsController) handlePullRequestEvent(logger logging.SimpleLogging, baseRepo models.Repo, headRepo models.Repo, pull models.PullRequest, user models.User, eventType models.PullRequestEventType, timestamp time.Time) HttpResponse {
	if !e.RepoAllowlistChecker.IsAllowlisted(baseRepo.FullName, baseRepo.VCSHost.Hostname) {
		// If the repo isn't allowlisted and we receive an opened pull request
		// event we comment back on the pull request that the repo isn't
		// allowlisted. This is because the user might be expecting Atlantis to
		// autoplan. For other events, we just ignore them.
		if eventType == models.OpenedPullEvent {
			e.commentNotAllowlisted(baseRepo, pull.Num)
		}

		err := errors.New(fmt.Sprintf("Pull request event from non-allowlisted repo \"%s/%s\"", baseRepo.VCSHost.Hostname, baseRepo.FullName))

		return HttpResponse{
			body: err.Error(),
			err: HttpError{
				code: http.StatusForbidden,
				err:  err,
			},
		}
	}

	switch eventType {
	case models.OpenedPullEvent, models.UpdatedPullEvent:
		// If the pull request was opened or updated, we will try to autoplan.

		// Respond with success and then actually execute the command asynchronously.
		// We use a goroutine so that this function returns and the connection is
		// closed.
		if !e.TestingMode {
			go e.CommandRunner.RunAutoplanCommand(logger, baseRepo, headRepo, pull, user, timestamp)
		} else {
			// When testing we want to wait for everything to complete.
			e.CommandRunner.RunAutoplanCommand(logger, baseRepo, headRepo, pull, user, timestamp)
		}
		return HttpResponse{
			body: "Processing...",
		}
	case models.ClosedPullEvent:
		// If the pull request was closed, we delete locks.
		if err := e.PullCleaner.CleanUpPull(baseRepo, pull); err != nil {
			return HttpResponse{
				body: err.Error(),
				err: HttpError{
					code: http.StatusForbidden,
					err:  err,
				},
			}
		}
		logger.Infof("deleted locks and workspace for repo %s, pull %d", baseRepo.FullName, pull.Num)
		return HttpResponse{
			body: "Pull request cleaned successfully",
		}
	case models.OtherPullEvent:
		// Else we ignore the event.
		return HttpResponse{
			body: "Ignoring non-actionable pull request event",
		}
	}
	return HttpResponse{}
}

func (e *VCSEventsController) handleGitlabPost(w http.ResponseWriter, r *http.Request) {
	event, err := e.GitlabRequestParserValidator.ParseAndValidate(r, e.GitlabWebhookSecret)
	if err != nil {
		e.respond(w, logging.Warn, http.StatusBadRequest, err.Error())
		return
	}
	e.Logger.Debugf("request valid")

	switch event := event.(type) {
	case gitlab.MergeCommentEvent:
		e.Logger.Debugf("handling as comment event")
		e.HandleGitlabCommentEvent(w, event)
	case gitlab.MergeEvent:
		e.Logger.Debugf("handling as pull request event")
		e.HandleGitlabMergeRequestEvent(w, event)
	case gitlab.CommitCommentEvent:
		e.Logger.Debugf("comments on commits are not supported, only comments on merge requests")
		e.respond(w, logging.Debug, http.StatusOK, "Ignoring comment on commit event")
	default:
		e.respond(w, logging.Debug, http.StatusOK, "Ignoring unsupported event")
	}

}

// HandleGitlabCommentEvent handles comment events from GitLab where Atlantis
// commands can come from. It's exported to make testing easier.
func (e *VCSEventsController) HandleGitlabCommentEvent(w http.ResponseWriter, event gitlab.MergeCommentEvent) {
	// todo: can gitlab return the pull request here too?
	baseRepo, headRepo, user, err := e.Parser.ParseGitlabMergeRequestCommentEvent(event)
	if err != nil {
		e.respond(w, logging.Error, http.StatusBadRequest, "Error parsing webhook: %s", err)
		return
	}
	eventTimestamp := time.Now()
	resp := e.handleCommentEvent(e.Logger, baseRepo, &headRepo, nil, user, event.MergeRequest.IID, event.ObjectAttributes.Note, models.Gitlab, eventTimestamp)

	//TODO: move this to the outer most function similar to github
	lvl := logging.Debug
	code := http.StatusOK
	msg := resp.body
	if resp.err.code != 0 {
		lvl = logging.Error
		code = resp.err.code
		msg = resp.err.err.Error()
	}
	e.respond(w, lvl, code, msg)
}

func (e *VCSEventsController) handleCommentEvent(logger logging.SimpleLogging, baseRepo models.Repo, maybeHeadRepo *models.Repo, maybePull *models.PullRequest, user models.User, pullNum int, comment string, vcsHost models.VCSHostType, timestamp time.Time) HttpResponse {
	parseResult := e.CommentParser.Parse(comment, vcsHost)
	if parseResult.Ignore {
		truncated := comment
		truncateLen := 40
		if len(truncated) > truncateLen {
			truncated = comment[:truncateLen] + "..."
		}
		return HttpResponse{
			body: fmt.Sprintf("Ignoring non-command comment: %q", truncated),
		}
	}
	logger.Infof("parsed comment as %s", parseResult.Command)

	// At this point we know it's a command we're not supposed to ignore, so now
	// we check if this repo is allowed to run commands in the first place.
	if !e.RepoAllowlistChecker.IsAllowlisted(baseRepo.FullName, baseRepo.VCSHost.Hostname) {
		e.commentNotAllowlisted(baseRepo, pullNum)

		err := errors.New("Repo not allowlisted")
		return HttpResponse{
			body: err.Error(),
			err: HttpError{
				err:  err,
				code: http.StatusForbidden,
			},
		}
	}

	// If the command isn't valid or doesn't require processing, ex.
	// "atlantis help" then we just comment back immediately.
	// We do this here rather than earlier because we need access to the pull
	// variable to comment back on the pull request.
	if parseResult.CommentResponse != "" {
		if err := e.VCSClient.CreateComment(baseRepo, pullNum, parseResult.CommentResponse, ""); err != nil {
			logger.Errorf("unable to comment on pull request: %s", err)
		}
		return HttpResponse{
			body: "Commenting back on pull request",
		}
	}

	logger.Debugf("executing command")
	if !e.TestingMode {
		// Respond with success and then actually execute the command asynchronously.
		// We use a goroutine so that this function returns and the connection is
		// closed.
		go e.CommandRunner.RunCommentCommand(logger, baseRepo, maybeHeadRepo, maybePull, user, pullNum, parseResult.Command, timestamp)
	} else {
		// When testing we want to wait for everything to complete.
		e.CommandRunner.RunCommentCommand(logger, baseRepo, maybeHeadRepo, maybePull, user, pullNum, parseResult.Command, timestamp)
	}

	return HttpResponse{
		body: "Processing...",
	}
}

// HandleGitlabMergeRequestEvent will delete any locks associated with the pull
// request if the event is a merge request closed event. It's exported to make
// testing easier.
func (e *VCSEventsController) HandleGitlabMergeRequestEvent(w http.ResponseWriter, event gitlab.MergeEvent) {
	pull, pullEventType, baseRepo, headRepo, user, err := e.Parser.ParseGitlabMergeRequestEvent(event)
	if err != nil {
		e.respond(w, logging.Error, http.StatusBadRequest, "Error parsing webhook: %s", err)
		return
	}
	e.Logger.Infof("identified event as type %q", pullEventType.String())
	eventTimestamp := time.Now()
	resp := e.handlePullRequestEvent(e.Logger, baseRepo, headRepo, pull, user, pullEventType, eventTimestamp)

	//TODO: move this to the outer most function similar to github
	lvl := logging.Debug
	code := http.StatusOK
	msg := resp.body
	if resp.err.code != 0 {
		lvl = logging.Error
		code = resp.err.code
		msg = resp.err.err.Error()
	}
	e.respond(w, lvl, code, msg)
}

// HandleAzureDevopsPullRequestCommentedEvent handles comment events from Azure DevOps where Atlantis
// commands can come from. It's exported to make testing easier.
// Sometimes we may want data from the parent azuredevops.Event struct, so we handle type checking here.
// Requires Resource Version 2.0 of the Pull Request Commented On webhook payload.
func (e *VCSEventsController) HandleAzureDevopsPullRequestCommentedEvent(w http.ResponseWriter, event *azuredevops.Event, azuredevopsReqID string) {
	resource, ok := event.Resource.(*azuredevops.GitPullRequestWithComment)
	if !ok || event.PayloadType != azuredevops.PullRequestCommentedEvent {
		e.respond(w, logging.Error, http.StatusBadRequest, "Event.Resource is nil or received bad event type %v; %s", event.Resource, azuredevopsReqID)
		return
	}

	if resource.Comment == nil {
		e.respond(w, logging.Debug, http.StatusOK, "Ignoring comment event since no comment is linked to payload; %s", azuredevopsReqID)
		return
	}
	strippedComment := bluemonday.StrictPolicy().SanitizeBytes([]byte(*resource.Comment.Content))

	if resource.PullRequest == nil {
		e.respond(w, logging.Debug, http.StatusOK, "Ignoring comment event since no pull request is linked to payload; %s", azuredevopsReqID)
		return
	}

	createdBy := resource.PullRequest.GetCreatedBy()
	user := models.User{Username: createdBy.GetUniqueName()}
	baseRepo, err := e.Parser.ParseAzureDevopsRepo(resource.PullRequest.GetRepository())
	if err != nil {
		e.respond(w, logging.Error, http.StatusBadRequest, "Error parsing pull request repository field: %s; %s", err, azuredevopsReqID)
		return
	}
	eventTimestamp := time.Now()
	resp := e.handleCommentEvent(e.Logger, baseRepo, nil, nil, user, resource.PullRequest.GetPullRequestID(), string(strippedComment), models.AzureDevops, eventTimestamp)

	//TODO: move this to the outer most function similar to github
	lvl := logging.Debug
	code := http.StatusOK
	msg := resp.body
	if resp.err.code != 0 {
		lvl = logging.Error
		code = resp.err.code
		msg = resp.err.err.Error()
	}
	e.respond(w, lvl, code, msg)
}

// HandleAzureDevopsPullRequestEvent will delete any locks associated with the pull
// request if the event is a pull request closed event. It's exported to make
// testing easier.
func (e *VCSEventsController) HandleAzureDevopsPullRequestEvent(w http.ResponseWriter, event *azuredevops.Event, azuredevopsReqID string) {
	prText := event.Message.GetText()
	ignoreEvents := []string{
		"changed the reviewer list",
		"approved pull request",
		"has approved and left suggestions",
		"is waiting for the author",
		"rejected pull request",
		"voted on pull request",
	}
	for _, s := range ignoreEvents {
		if strings.Contains(prText, s) {
			msg := fmt.Sprintf("pull request updated event is not a supported type [%s]", s)
			e.respond(w, logging.Debug, http.StatusOK, "%s: %s", msg, azuredevopsReqID)
			return
		}
	}

	pull, pullEventType, baseRepo, headRepo, user, err := e.Parser.ParseAzureDevopsPullEvent(*event)
	if err != nil {
		e.respond(w, logging.Error, http.StatusBadRequest, "Error parsing pull data: %s %s", err, azuredevopsReqID)
		return
	}
	e.Logger.Infof("identified event as type %q", pullEventType.String())
	eventTimestamp := time.Now()
	resp := e.handlePullRequestEvent(e.Logger, baseRepo, headRepo, pull, user, pullEventType, eventTimestamp)

	//TODO: move this to the outer most function similar to github
	lvl := logging.Debug
	code := http.StatusOK
	msg := resp.body
	if resp.err.code != 0 {
		lvl = logging.Error
		code = resp.err.code
		msg = resp.err.err.Error()
	}
	e.respond(w, lvl, code, msg)
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
	e.Logger.Log(lvl, response)
	w.WriteHeader(code)
	fmt.Fprintln(w, response)
}

// commentNotAllowlisted comments on the pull request that the repo is not
// allowlisted unless allowlist error comments are disabled.
func (e *VCSEventsController) commentNotAllowlisted(baseRepo models.Repo, pullNum int) {
	errMsg := "```\nError: This repo is not allowlisted for Atlantis.\n```"
	if err := e.VCSClient.CreateComment(baseRepo, pullNum, errMsg, ""); err != nil {
		e.Logger.Errorf("unable to comment on pull request: %s", err)
	}
}
