package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/http"
	"github.com/runatlantis/atlantis/server/logging"
)

func NewCommentEvent(
	commentParser events.CommentParsing,
	repoAllowlistChecker *events.RepoAllowlistChecker,
	vcsClient vcs.Client,
	CommandRunner events.CommandRunner,
) *CommentEvent {
	return &CommentEvent{
		CommentParser: commentParser,
		CommandHandler: &asyncHandler{
			CommandRunner: CommandRunner,
		},
		RepoAllowlistChecker: repoAllowlistChecker,
		VCSClient:            vcsClient,
	}

}

func NewCommentEventWithCommandHandler(
	commentParser events.CommentParsing,
	repoAllowlistChecker *events.RepoAllowlistChecker,
	vcsClient vcs.Client,
	commandHandler commandHandler,
) *CommentEvent {
	return &CommentEvent{
		CommentParser:        commentParser,
		CommandHandler:       commandHandler,
		RepoAllowlistChecker: repoAllowlistChecker,
		VCSClient:            vcsClient,
	}

}

// commandHandler is the handler responsible for running a specific command
// after it's been parsed from a comment.
type commandHandler interface {
	Handle(ctx context.Context, request *http.CloneableRequest, input CommentEventInput, command *command.Comment) error
}

type asyncHandler struct {
	CommandRunner events.CommandRunner
}

func (h *asyncHandler) Handle(ctx context.Context, _ *http.CloneableRequest, input CommentEventInput, command *command.Comment) error {
	go h.CommandRunner.RunCommentCommand(
		input.Logger,
		input.BaseRepo,
		input.MaybeHeadRepo,
		input.MaybePull,
		input.User,
		input.PullNum,
		command,
		input.Timestamp,
	)
	return nil
}

type CommentEventInput struct {
	//TODO: enforce the existence of this so that we can eliminate
	// a number of other fields in this struct
	MaybePull *models.PullRequest

	BaseRepo      models.Repo
	MaybeHeadRepo *models.Repo
	User          models.User
	PullNum       int
	Comment       string
	VCSHost       models.VCSHostType
	Timestamp     time.Time
	Logger        logging.SimpleLogging
}

type CommentEvent struct {
	CommentParser        events.CommentParsing
	CommandHandler       commandHandler
	RepoAllowlistChecker *events.RepoAllowlistChecker
	VCSClient            vcs.Client
}

func (h *CommentEvent) commentNotAllowlisted(baseRepo models.Repo, pullNum int, logger logging.SimpleLogging) {
	errMsg := "```\nError: This repo is not allowlisted for Atlantis.\n```"
	if err := h.VCSClient.CreateComment(baseRepo, pullNum, errMsg, ""); err != nil {
		logger.Errorf("unable to comment on pull request: %s", err)
	}
}

func (h *CommentEvent) Handle(ctx context.Context, request *http.CloneableRequest, input CommentEventInput) error {
	comment := input.Comment
	vcsHost := input.VCSHost
	logger := input.Logger
	baseRepo := input.BaseRepo
	pullNum := input.PullNum

	parseResult := h.CommentParser.Parse(comment, vcsHost)
	if parseResult.Ignore {
		truncated := comment
		truncateLen := 40
		if len(truncated) > truncateLen {
			truncated = comment[:truncateLen] + "..."
		}

		logger.Warnf("Ignoring non-command comment: %q", truncated)
		return nil
	}
	logger.Infof("parsed comment as %s", parseResult.Command)

	// At this point we know it's a command we're not supposed to ignore, so now
	// we check if this repo is allowed to run commands in the first plach.
	if !h.RepoAllowlistChecker.IsAllowlisted(baseRepo.FullName, baseRepo.VCSHost.Hostname) {
		h.commentNotAllowlisted(baseRepo, pullNum, logger)

		return fmt.Errorf("comment event from non-allowlisted repo \"%s/%s\"", baseRepo.VCSHost.Hostname, baseRepo.FullName)
	}

	// If the command isn't valid or doesn't require processing, ex.
	// "atlantis help" then we just comment back immediately.
	// We do this here rather than earlier because we need access to the pull
	// variable to comment back on the pull request.
	if parseResult.CommentResponse != "" {
		if err := h.VCSClient.CreateComment(baseRepo, pullNum, parseResult.CommentResponse, ""); err != nil {
			logger.Errorf("unable to comment on pull request: %s", err)
		}
		return nil
	}

	return h.CommandHandler.Handle(ctx, request, input, parseResult.Command)
}
