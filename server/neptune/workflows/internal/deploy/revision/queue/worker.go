package queue

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	internalContext "github.com/runatlantis/atlantis/server/neptune/context"
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

type githubActivities interface {
	UpdateCheckRun(ctx context.Context, request activities.UpdateCheckRunRequest) (activities.UpdateCheckRunResponse, error)
	CompareCommit(ctx context.Context, request activities.CompareCommitRequest) (activities.CompareCommitResponse, error)
}

type workerActivites interface {
	dbActivities
	githubActivities
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
	Activity                workerActivites
	Repo                    github.Repo

	// mutable
	state            WorkerState
	LatestDeployment *root.DeploymentInfo
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

		ctx := workflow.WithValue(ctx, internalContext.SHAKey, msg.Revision)
		ctx = workflow.WithValue(ctx, internalContext.DeploymentIDKey, msg.ID)

		// This should only happen on startup
		// If we fail to fetch the latest deployment, log and exit the workflow
		if w.LatestDeployment == nil {
			w.LatestDeployment, err = w.fetchLatestDeployment(ctx, msg)
			if err != nil {
				logger.Error(ctx, fmt.Sprint("Unable to fetch latest deployment, worker is shutting down", err.Error()))
				return
			}
		}

		// TODO: Skip revision validation if it's a Force Apply
		aheadBy, err := w.deployRequestRevisionIsAheadBy(ctx, msg)
		if err != nil {
			logger.Error(ctx, fmt.Sprintf("Validating deploy request revision %s: %s", msg.Revision, err.Error()))

			// TODO: Update the checkrun with relevant info
			continue
		}

		switch {
		case aheadBy == 0:
			// TODO: Update checkrun with relevant Info [Revision is already deployed]
			logger.Info(ctx, fmt.Sprintf("Deployed Revision is identical to the Deploy Request Revision: %s, skipping deploy", msg.Revision))
			continue
		case aheadBy < 0:
			// TODO: Update checkrun with relevant Info [Revision is behind the current deployed version]
			logger.Info(ctx, fmt.Sprintf("Deployed Revision is ahead of the Deploy Request Revision: %s, skipping deploy", msg.Revision))
			continue
		}

		// Run the terraform workflow if revision is ahead by at least 1 commit
		err = w.TerraformWorkflowRunner.Run(ctx, msg)
		if err != nil {
			logger.Error(ctx, "failed to deploy revision, moving to next one")
		}

		latestDeployment, err := w.persistLatestDeployment(ctx, msg)
		if err != nil {
			logger.Error(ctx, "failed to persist latest deploy job")
		}

		// Update the latest deployment in memory
		w.LatestDeployment = latestDeployment
	}
}

func (w *Worker) deployRequestRevisionIsAheadBy(ctx workflow.Context, deploymentInfo terraform.DeploymentInfo) (int, error) {
	// skip compare commit if deploy request revision is the same as latest deployed revision
	if deploymentInfo.Revision == w.LatestDeployment.Revision {
		return 0, nil
	}

	var compareCommitResp activities.CompareCommitResponse
	err := workflow.ExecuteActivity(ctx, w.Activity.CompareCommit, activities.CompareCommitRequest{
		DeployRequestRevision:  deploymentInfo.Revision,
		LatestDeployedRevision: w.LatestDeployment.Revision,
		Repo:                   w.Repo,
	}).Get(ctx, &compareCommitResp)
	if err != nil {
		return 0, errors.Wrap(err, "comparing revision")
	}

	return compareCommitResp.DeployRequestRevisionAheadBy, nil
}

func (w *Worker) fetchLatestDeployment(ctx workflow.Context, deploymentInfo terraform.DeploymentInfo) (*root.DeploymentInfo, error) {
	// Fetch latest deployment
	var resp activities.FetchLatestDeploymentResponse
	err := workflow.ExecuteActivity(ctx, w.Activity.FetchLatestDeployment, activities.FetchLatestDeploymentRequest{
		FullRepositoryName: w.Repo.GetFullName(),
		RootName:           deploymentInfo.Root.Name,
	}).Get(ctx, &resp)
	if err != nil {
		return nil, errors.Wrap(err, "fetching latest deployment")
	}

	return resp.DeploymentInfo, nil
}

func (w *Worker) persistLatestDeployment(ctx workflow.Context, deploymentInfo terraform.DeploymentInfo) (*root.DeploymentInfo, error) {
	latestDeploymentInfo := root.DeploymentInfo{
		ID:         deploymentInfo.ID.String(),
		CheckRunID: deploymentInfo.CheckRunID,
		Revision:   deploymentInfo.Revision,
		Root:       deploymentInfo.Root,
		Repo:       w.Repo,
	}
	err := workflow.ExecuteActivity(ctx, w.Activity.StoreLatestDeployment, activities.StoreLatestDeploymentRequest{
		DeploymentInfo: latestDeploymentInfo,
	}).Get(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "persisting deployment info")
	}
	return &latestDeploymentInfo, nil
}

func (w *Worker) GetState() WorkerState {
	return w.state
}
