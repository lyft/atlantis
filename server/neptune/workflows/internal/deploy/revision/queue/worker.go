package queue

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	internal_context "github.com/runatlantis/atlantis/server/neptune/context"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/config/logger"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/root"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type terraformWorkflowRunner interface {
	Run(ctx workflow.Context, deploymentInfo terraform.DeploymentInfo) error
}

type dbActivities interface {
	FetchLatestDeployment(ctx context.Context, request activities.FetchLatestDeploymentRequest) (activities.FetchLatestDeploymentResponse, error)
	StoreLatestDeployment(ctx context.Context, request activities.StoreLatestDeploymentRequest) error
}

type WorkerState string

const (
	WaitingWorkerState  WorkerState = "waiting"
	WorkingWorkerState  WorkerState = "working"
	CompleteWorkerState WorkerState = "complete"
)

type Worker struct {
	Queue                   *Queue
	TerraformWorkflowRunner terraformWorkflowRunner
	DbActivities            dbActivities
	Repo                    github.Repo
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

		ctx := workflow.WithValue(ctx, internal_context.SHAKey, msg.Revision)
		ctx = workflow.WithValue(ctx, internal_context.DeploymentIDKey, msg.ID)

		isValid, err := w.isRequestRevisionValid(ctx, msg)
		if err != nil {
			logger.Error(ctx, fmt.Sprintf("Validating deploy request revision %s: %s", msg.Revision, err.Error()))
		}

		if !isValid {
			logger.Info(ctx, fmt.Sprintf("ID: %s Deploy Request Revision: %s is not valid", msg.ID, msg.Revision))
		}

		err = w.TerraformWorkflowRunner.Run(ctx, msg)
		if err != nil {
			logger.Error(ctx, "failed to deploy revision, moving to next one")
		}

		err = w.persistLatestDeployment(ctx, msg)
		if err != nil {
			logger.Error(ctx, "failed to persist latest deploy job")
		}
	}
}

func (w *Worker) isRequestRevisionValid(ctx workflow.Context, deploymentInfo terraform.DeploymentInfo) (bool, error) {
	// Fetch latest deployment
	var resp activities.FetchLatestDeploymentResponse
	err := workflow.ExecuteActivity(ctx, w.DbActivities.FetchLatestDeployment, activities.FetchLatestDeploymentRequest{
		FullRepositoryName: w.Repo.GetFullName(),
		RootName:           deploymentInfo.Root.Name,
	}).Get(ctx, &resp)
	if err != nil {
		return false, errors.Wrap(err, "fetching latest deployment")
	}

	// TODO: Validate revision by comparing the deploy request revision with the latest deployed
	return true, nil

}

func (w *Worker) persistLatestDeployment(ctx workflow.Context, deploymentInfo terraform.DeploymentInfo) error {
	err := workflow.ExecuteActivity(ctx, w.DbActivities.StoreLatestDeployment, activities.StoreLatestDeploymentRequest{
		DeploymentInfo: root.DeploymentInfo{
			ID:         deploymentInfo.ID.String(),
			CheckRunID: deploymentInfo.CheckRunID,
			Revision:   deploymentInfo.Revision,
			Root:       deploymentInfo.Root,
			Repo: root.Repo{
				Name:  w.Repo.Name,
				Owner: w.Repo.Owner,
			},
		},
	}).Get(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "persisting deployment info")
	}
	return nil
}

func (w *Worker) GetState() WorkerState {
	return w.state
}
