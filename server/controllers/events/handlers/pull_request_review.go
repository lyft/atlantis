package handlers

import (
	"context"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/http"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event"
	"time"
)

type PRReviewCommandRunner interface {
	RunPRReviewCommand(ctx context.Context, repo models.Repo, pull models.PullRequest, user models.User, timestamp time.Time, installationToken int64)
}

type PullRequestReviewEventHandler struct {
	PRReviewCommandRunner PRReviewCommandRunner
}

func (p PullRequestReviewEventHandler) Handle(ctx context.Context, event event.PullRequestReview) {
	p.PRReviewCommandRunner.RunPRReviewCommand(
		ctx,
		event.Repo,
		event.Pull,
		event.User,
		event.Timestamp,
		event.InstallationToken,
	)
}

type asyncPullRequestReviewEvent struct {
	handler *PullRequestReviewEventHandler
}

func NewPullRequestReviewEvent(prReviewCommandRunner PRReviewCommandRunner) *asyncPullRequestReviewEvent {
	return &asyncPullRequestReviewEvent{
		handler: &PullRequestReviewEventHandler{
			PRReviewCommandRunner: prReviewCommandRunner,
		},
	}
}

func (a asyncPullRequestReviewEvent) Handle(ctx context.Context, event event.PullRequestReview, _ *http.BufferedRequest) error {
	go func() {
		// Passing background context to avoid context cancellation since the parent goroutine does not wait for this goroutine to finish execution.
		a.handler.Handle(context.Background(), event)
	}()
	return nil
}
