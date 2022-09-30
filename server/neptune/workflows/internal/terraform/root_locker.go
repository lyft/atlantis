package terraform

import (
	"context"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/root"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"go.temporal.io/sdk/workflow"
)

const UnlockSignalName = "unlock"

type storeActivities interface {
	FetchLatestDeployment(ctx context.Context, request activities.FetchLatestDeploymentRequest) (activities.FetchLatestDeploymentResponse, error)
}

type LockNotifier func(lockState *state.Lock) error

type RootLocker struct {
	request  Request
	notifier LockNotifier
	ga       githubActivities
	sa       storeActivities
}

func NewRootLocker(request Request, ga githubActivities, sa storeActivities, notifier LockNotifier) *RootLocker {
	return &RootLocker{
		request:  request,
		notifier: notifier,
		ga:       ga,
		sa:       sa,
	}
}

func (r *RootLocker) Lock(ctx workflow.Context) error {
	// Fetch root's latest revision
	var fetchLatestDeploymentResponse *activities.FetchLatestDeploymentResponse
	err := workflow.ExecuteActivity(ctx, r.sa.FetchLatestDeployment, activities.FetchLatestDeploymentRequest{
		RepositoryName: r.request.Repo.Name,
		RootName:       r.request.Root.Name,
	}).Get(ctx, &fetchLatestDeploymentResponse)

	if err != nil {
		return err
	}

	// Compare with requested revision
	var compareCommitsResponse *activities.CompareCommitsResponse
	err = workflow.ExecuteActivity(ctx, r.ga.CompareCommits, activities.CompareCommitsRequest{
		OldCommit: fetchLatestDeploymentResponse.Revision,
		NewCommit: r.request.Revision,
	}).Get(ctx, &compareCommitsResponse)

	if err != nil {
		return err
	}

	// Notify parent workflow + wait for unlock signal if request is from diverged commit that was merged
	if compareCommitsResponse.IsDiverged && r.request.Root.Trigger == root.MergeTrigger {
		err = r.notifier(&state.Lock{Locked: true})
		if err != nil {
			return err
		}
		signalChan := workflow.GetSignalChannel(ctx, UnlockSignalName)
		var unlock bool
		_ = signalChan.Receive(ctx, &unlock)
	}
	return nil
}
