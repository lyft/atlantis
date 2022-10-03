package terraform

import (
	"context"
	"errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/root"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"go.temporal.io/sdk/workflow"
)

const (
	DivergedStatus = "diverged"
)

type storeActivities interface {
	FetchLatestDeployment(ctx context.Context, request activities.FetchLatestDeploymentRequest) (activities.FetchLatestDeploymentResponse, error)
}

type LockNotifier func(lockState *state.Lock) error

type RootLocker struct {
	Request  Request
	Notifier LockNotifier
	Ga       githubActivities
	Sa       storeActivities
}

func (r *RootLocker) Lock(ctx workflow.Context) error {
	// Fetch root's latest revision
	var fetchLatestDeploymentResponse *activities.FetchLatestDeploymentResponse
	err := workflow.ExecuteActivity(ctx, r.Sa.FetchLatestDeployment, activities.FetchLatestDeploymentRequest{
		RepositoryName: r.Request.Repo.Name,
		RootName:       r.Request.Root.Name,
	}).Get(ctx, &fetchLatestDeploymentResponse)

	if err != nil {
		return err
	}

	// Compare with requested revision
	var compareCommitsResponse *activities.CompareCommitsResponse
	err = workflow.ExecuteActivity(ctx, r.Ga.CompareCommits, activities.CompareCommitsRequest{
		Repo:      r.Request.Repo,
		OldCommit: fetchLatestDeploymentResponse.Revision,
		NewCommit: r.Request.Revision,
	}).Get(ctx, &compareCommitsResponse)

	if err != nil {
		return err
	}

	// Notify parent workflow + wait for unlock signal if request is from diverged commit that was merged
	if compareCommitsResponse.Status == DivergedStatus && r.Request.Root.Trigger == root.MergeTrigger {
		return r.RequestAndWaitForUnlockSignal(ctx)
	}
	return nil
}

func (r *RootLocker) RequestAndWaitForUnlockSignal(ctx workflow.Context) error {
	err := r.Notifier(&state.Lock{Locked: true})
	if err != nil {
		return err
	}
	signalChan := workflow.GetSignalChannel(ctx, UnlockSignalName)
	var unlockRequest UnlockSignalRequest
	_ = signalChan.Receive(ctx, &unlockRequest)
	if !unlockRequest.Unlock {
		return errors.New("received a false unlock signal, this is a bug")
	}
	return nil
}
