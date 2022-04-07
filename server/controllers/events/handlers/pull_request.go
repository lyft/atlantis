package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/runatlantis/atlantis/server/controllers/events/errors"
	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/http"
	"github.com/runatlantis/atlantis/server/logging"
)

type PullRequestEventInput struct {
	Logger    logging.SimpleLogging
	Pull      models.PullRequest
	User      models.User
	EventType models.PullRequestEventType
	Timestamp time.Time
}

type eventTypeHandler interface {
	Handle(ctx context.Context, request *http.CloneableRequest, input PullRequestEventInput) error
}

type AsyncAutoplanner struct {
	CommandRunner events.CommandRunner
}

func (p *AsyncAutoplanner) Handle(ctx context.Context, _ *http.CloneableRequest, input PullRequestEventInput) error {
	go p.CommandRunner.RunAutoplanCommand(
		input.Logger,
		input.Pull.BaseRepo,
		input.Pull.HeadRepo,
		input.Pull,
		input.User,
		input.Timestamp,
	)

	return nil
}

type PullCleaner struct {
	PullCleaner events.PullCleaner
}

func (c *PullCleaner) Handle(ctx context.Context, _ *http.CloneableRequest, input PullRequestEventInput) error {
	if err := c.PullCleaner.CleanUpPull(input.Pull.BaseRepo, input.Pull); err != nil {
		return err
	}

	input.Logger.Infof("deleted locks and workspace for repo %s, pull %d", input.Pull.BaseRepo.FullName, input.Pull.Num)

	return nil
}

func NewPullRequestEvent(
	repoAllowlistChecker *events.RepoAllowlistChecker,
	vcsClient vcs.Client,
	pullCleaner events.PullCleaner,
	commandRunner events.CommandRunner) *PullRequestEvent {
	asyncAutoplanner := &AsyncAutoplanner{
		CommandRunner: commandRunner,
	}
	return &PullRequestEvent{
		RepoAllowlistChecker:    repoAllowlistChecker,
		VCSClient:               vcsClient,
		OpenedPullEventHandler:  asyncAutoplanner,
		UpdatedPullEventHandler: asyncAutoplanner,
		ClosedPullEventHandler: &PullCleaner{
			PullCleaner: pullCleaner,
		},
	}
}

func NewPullRequestEventWithEventTypeHandlers(
	repoAllowlistChecker *events.RepoAllowlistChecker,
	vcsClient vcs.Client,
	openedPullEventHandler eventTypeHandler,
	updatedPullEventHandler eventTypeHandler,
	closedPullEventHandler eventTypeHandler,
) *PullRequestEvent {
	return &PullRequestEvent{
		RepoAllowlistChecker:    repoAllowlistChecker,
		VCSClient:               vcsClient,
		OpenedPullEventHandler:  openedPullEventHandler,
		UpdatedPullEventHandler: updatedPullEventHandler,
		ClosedPullEventHandler:  closedPullEventHandler,
	}
}

type PullRequestEvent struct {
	RepoAllowlistChecker *events.RepoAllowlistChecker
	VCSClient            vcs.Client

	// Delegate Handlers
	OpenedPullEventHandler  eventTypeHandler
	UpdatedPullEventHandler eventTypeHandler
	ClosedPullEventHandler  eventTypeHandler
}

func (h *PullRequestEvent) commentNotAllowlisted(baseRepo models.Repo, pullNum int, logger logging.SimpleLogging) {
	errMsg := "```\nError: This repo is not allowlisted for Atlantis.\n```"
	if err := h.VCSClient.CreateComment(baseRepo, pullNum, errMsg, ""); err != nil {
		logger.Errorf("unable to comment on pull request: %s", err)
	}
}

func (h *PullRequestEvent) Handle(ctx context.Context, request *http.CloneableRequest, input PullRequestEventInput) error {
	pull := input.Pull
	baseRepo := pull.BaseRepo
	logger := input.Logger
	eventType := input.EventType

	if !h.RepoAllowlistChecker.IsAllowlisted(baseRepo.FullName, baseRepo.VCSHost.Hostname) {
		// If the repo isn't allowlisted and we receive an opened pull request
		// event we comment back on the pull request that the repo isn't
		// allowlisted. This is because the user might be expecting Atlantis to
		// autoplan. For other events, we just ignore them.
		if eventType == models.OpenedPullEvent {
			h.commentNotAllowlisted(baseRepo, pull.Num, logger)
		}

		return fmt.Errorf("Pull request event from non-allowlisted repo \"%s/%s\"", baseRepo.VCSHost.Hostname, baseRepo.FullName)
	}

	switch eventType {
	case models.OpenedPullEvent:
		return h.OpenedPullEventHandler.Handle(ctx, request, input)
	case models.UpdatedPullEvent:
		return h.UpdatedPullEventHandler.Handle(ctx, request, input)
	case models.ClosedPullEvent:
		return h.ClosedPullEventHandler.Handle(ctx, request, input)
	case models.OtherPullEvent:
		return &errors.UnsupportedEventTypeError{Msg: "Unsupported event type made it through, this is likely a bug in the code."}
	}
	return nil
}
