package converter

import (
	"context"
	"fmt"
	"time"

	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"

	"github.com/google/go-github/v45/github"
	"github.com/runatlantis/atlantis/server/models"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event"
)

type PullFetcher interface {
	Fetch(ctx context.Context, installationToken int64, repoOwner string, repoName string, prNum int) (*github.PullRequest, error)
}

type PullEventConverter struct {
	PullConverter PullConverter
	AllowDraftPRs bool
	PullFetcher   PullFetcher
}

// Converts a github pull request event to our internal representation
func (e PullEventConverter) Convert(ctx context.Context, pullEvent *github.PullRequestEvent) (event.PullRequest, error) {
	if pullEvent.PullRequest == nil {
		return event.PullRequest{}, fmt.Errorf("pull_request is null")
	}
	pullFromEvent, err := e.PullConverter.Convert(pullEvent.PullRequest)
	if err != nil {
		return event.PullRequest{}, err
	}
	installationToken := githubapp.GetInstallationIDFromEvent(pullEvent)
	// fetch the latest pull request state in case of out of order events
	latestPRState, err := e.PullFetcher.Fetch(ctx, installationToken, pullFromEvent.HeadRepo.Owner, pullFromEvent.HeadRepo.Name, pullFromEvent.Num)
	if err != nil {
		return event.PullRequest{}, errors.Wrap(err, "fetching latest pull request state")
	}
	pull, err := e.PullConverter.Convert(latestPRState)
	if err != nil {
		return event.PullRequest{}, err
	}

	// Rely on action from original pull event to filter out irrelevant PR events
	action := pullEvent.GetAction()
	// If it's a draft PR we ignore it for auto-planning if configured to do so
	// however it's still possible for users to run plan on it manually via a
	// comment so if any draft PR is closed we still need to check if we need
	// to delete its locks.
	if pullEvent.GetPullRequest().GetDraft() && pullEvent.GetAction() != "closed" && !e.AllowDraftPRs {
		action = "other"
	}

	var pullEventType models.PullRequestEventType
	switch action {
	case "opened":
		pullEventType = models.OpenedPullEvent
	case "ready_for_review":
		// when an author takes a PR out of 'draft' state a 'ready_for_review'
		// event is triggered. We want atlantis to treat this as a freshly opened PR
		pullEventType = models.OpenedPullEvent
	case "synchronize":
		pullEventType = models.UpdatedPullEvent
	case "closed":
		pullEventType = models.ClosedPullEvent
	default:
		pullEventType = models.OtherPullEvent
	}

	eventTimestamp := time.Now()
	if latestPRState.UpdatedAt != nil {
		eventTimestamp = *latestPRState.UpdatedAt
	}
	return event.PullRequest{
		Pull:              pull,
		EventType:         pullEventType,
		User:              models.User{Username: latestPRState.GetUser().GetLogin()},
		Timestamp:         eventTimestamp,
		InstallationToken: installationToken,
	}, nil
}
