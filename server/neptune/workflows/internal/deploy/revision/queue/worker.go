package queue

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	internalContext "github.com/runatlantis/atlantis/server/neptune/context"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/config/logger"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/root"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const DeploymentInfoVersion = "1.0.0"

type terraformWorkflowRunner interface {
	Run(ctx workflow.Context, deploymentInfo terraform.DeploymentInfo) error
}

type dbActivities interface {
	FetchLatestDeployment(ctx context.Context, request activities.FetchLatestDeploymentRequest) (activities.FetchLatestDeploymentResponse, error)
	StoreLatestDeployment(ctx context.Context, request activities.StoreLatestDeploymentRequest) error
}

type revisionProcessor interface {
	Process(ctx workflow.Context, requestedDeployment terraform.DeploymentInfo, latestDeployment *root.DeploymentInfo) error
}

type workerActivities interface {
	dbActivities
}

type WorkerState string

const (
	WaitingWorkerState  WorkerState = "waiting"
	WorkingWorkerState  WorkerState = "working"
	CompleteWorkerState WorkerState = "complete"

	UnlockSignalName = "unlock"
)

type UnlockSignalRequest struct {
	User string
}

type Worker struct {
	Queue                   *Queue
	TerraformWorkflowRunner terraformWorkflowRunner
	Activities              workerActivities
	RevisionProcessor       revisionProcessor

	// mutable
	state WorkerState
}

// Work pops work off the queue and if the queue is empty,
// it waits for the queue to be non-empty or a cancelation signal
func (w *Worker) Work(ctx workflow.Context) {
	// set to complete once we return else callers could think we are still working based on the 'working' state.
	defer func() {
		w.state = CompleteWorkerState
	}()

	var latestDeployment *root.DeploymentInfo
	for {
		if w.Queue.IsEmpty() {
			w.state = WaitingWorkerState
		}

		err := workflow.Await(ctx, func() bool {
			return !w.Queue.IsEmpty()
		})

		if temporal.IsCanceledError(err) {
			logger.Info(ctx, "Received cancelled signal, worker is shutting down")
			return
		}

		if err != nil {
			logger.Error(ctx, fmt.Sprintf("Unknown error %s, worker is shutting down", err.Error()))
			return
		}

		w.state = WorkingWorkerState

		msg := w.Queue.Pop()

		ctx := workflow.WithValue(ctx, internalContext.SHAKey, msg.Revision)
		ctx = workflow.WithValue(ctx, internalContext.DeploymentIDKey, msg.ID)

		latestDeployment, err = w.fetchLatestDeployment(ctx, msg, latestDeployment)
		if err != nil {
			logger.Error(ctx, "unable to fetch latest deployment for root: %s, skipping deploy: %s", msg.Root.Name, err.Error())
			continue
		}
		err = w.RevisionProcessor.Process(ctx, msg, latestDeployment)
		if err != nil {
			logger.Error(ctx, "failed to process revision, moving to next one")
			continue
		}

		err = w.TerraformWorkflowRunner.Run(ctx, msg)
		if err != nil {
			logger.Error(ctx, "failed to deploy revision, moving to next one")
			continue
		}
		latestDeployment = w.buildLatestDeployment(msg)

		// TODO: Persist deployment on shutdown if it fails instead of blocking
		if err = w.persistLatestDeployment(ctx, latestDeployment); err != nil {
			logger.Error(ctx, "failed to persist latest deploy job")
		}
	}
}

func (w *Worker) buildLatestDeployment(deployRequest terraform.DeploymentInfo) *root.DeploymentInfo {
	return &root.DeploymentInfo{
		Version:    DeploymentInfoVersion,
		ID:         deployRequest.ID.String(),
		CheckRunID: deployRequest.CheckRunID,
		Revision:   deployRequest.Revision,
		Root:       deployRequest.Root,
		Repo:       deployRequest.Repo,
	}
}

func (w *Worker) fetchLatestDeployment(ctx workflow.Context, deploymentInfo terraform.DeploymentInfo, latestDeployment *root.DeploymentInfo) (*root.DeploymentInfo, error) {
	// Skip fetching latest deployment it it's already in memory
	if latestDeployment != nil {
		return latestDeployment, nil
	}
	var resp activities.FetchLatestDeploymentResponse
	err := workflow.ExecuteActivity(ctx, w.Activities.FetchLatestDeployment, activities.FetchLatestDeploymentRequest{
		FullRepositoryName: deploymentInfo.Repo.GetFullName(),
		RootName:           deploymentInfo.Root.Name,
	}).Get(ctx, &resp)
	if err != nil {
		return nil, errors.Wrap(err, "fetching latest deployment")
	}
	return resp.DeploymentInfo, nil
}

func (w *Worker) persistLatestDeployment(ctx workflow.Context, deploymentInfo *root.DeploymentInfo) error {
	err := workflow.ExecuteActivity(ctx, w.Activities.StoreLatestDeployment, activities.StoreLatestDeploymentRequest{
		DeploymentInfo: deploymentInfo,
	}).Get(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "persisting deployment info")
	}
	return nil
}

func (w *Worker) GetState() WorkerState {
	return w.state
}
