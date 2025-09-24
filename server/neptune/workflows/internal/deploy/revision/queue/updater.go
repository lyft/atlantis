package queue

import (
	"fmt"

	key "github.com/runatlantis/atlantis/server/neptune/context"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/notifier"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/lock"
	"go.temporal.io/sdk/workflow"
)

type CheckRunClient interface {
	CreateOrUpdate(ctx workflow.Context, deploymentID string, request notifier.GithubCheckRunRequest) (int64, error)
}

type LockStateUpdater struct {
	Activities          githubActivities
	GithubCheckRunCache CheckRunClient
}

func (u *LockStateUpdater) UpdateQueuedRevisions(ctx workflow.Context, queue *Deploy, repoFullName string) {
	queueLock := queue.GetLockState()
	infos := queue.GetOrderedMergedItems()

	var actions []github.CheckRunAction
	var summary string
	var revisionsSummary = queue.GetQueuedRevisionsSummary()
	state := github.CheckRunQueued
	if queueLock.Status == lock.LockedStatus {
		actions = append(actions, github.CreateUnlockAction())
		state = github.CheckRunActionRequired
		revisionLink := github.BuildRevisionURLMarkdown(repoFullName, queueLock.Revision)
		summary = fmt.Sprintf("This deploy is locked from a manual deployment for revision %s.  Unlock to proceed.\n%s", revisionLink, revisionsSummary)
	}

	for _, i := range infos {
		request := notifier.GithubCheckRunRequest{
			Title:   notifier.BuildDeployCheckRunTitle(i.Root.Name),
			Sha:     i.Commit.Revision,
			State:   state,
			Repo:    i.Repo,
			Summary: summary,
			Actions: actions,
		}

		workflow.GetLogger(ctx).Debug(fmt.Sprintf("Updating lock status for deployment id: %s", i.ID.String()))
		_, err := u.GithubCheckRunCache.CreateOrUpdate(ctx, i.ID.String(), request)

		if err != nil {
			workflow.GetLogger(ctx).Debug(fmt.Sprintf("updating check run for revision %s", i.Commit.Revision), key.ErrKey, err)
		}
	}
}
