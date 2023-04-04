package queue

import (
	"fmt"

	key "github.com/runatlantis/atlantis/server/neptune/context"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/config/logger"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/notifier"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"go.temporal.io/sdk/workflow"
)

type CheckRunClient interface {
	CreateOrUpdate(ctx workflow.Context, deploymentID string, request notifier.GithubCheckRunRequest) (int64, error)
}

type LockStateUpdater struct {
	Activities          githubActivities
	GithubCheckRunCache CheckRunClient
}

func (u *LockStateUpdater) UpdateQueuedRevisions(ctx workflow.Context, queue *Deploy) {
	lock := queue.GetLockState()
	infos := queue.GetOrderedMergedItems()

	var actions []github.CheckRunAction
	var summary string
	state := github.CheckRunQueued
	if lock.Status == LockedStatus {
		actions = append(actions, github.CreateUnlockAction())
		state = github.CheckRunActionRequired
		summary = fmt.Sprintf("This deploy is locked from a manual deployment for revision %s.  Unlock to proceed.", lock.Revision)
	}

	for _, i := range infos {
		request := notifier.GithubCheckRunRequest{
			Title:   terraform.BuildCheckRunTitle(i.Root.Name),
			Sha:     i.Commit.Revision,
			State:   state,
			Repo:    i.Repo,
			Summary: summary,
			Actions: actions,
		}
		logger.Debug(ctx, fmt.Sprintf("Updating lock status for deployment id: %s", i.ID.String()))

		_, err := u.GithubCheckRunCache.CreateOrUpdate(ctx, i.ID.String(), request)

		if err != nil {
			logger.Error(ctx, fmt.Sprintf("updating check run for revision %s", i.Commit.Revision), key.ErrKey, err)
		}
	}
}
