package queue

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/config"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/config/logger"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	"go.temporal.io/sdk/workflow"
)

type workerActivities interface {
	FetchLatestDeployment(ctx context.Context, request activities.FetchLatestDeploymentRequest) (activities.FetchLatestDeploymentResponse, error)
}

type WorkerState string

const (
	WaitingWorkerState = "waiting"
	WorkingWorkerState = "working"
)

type Worker struct {
	Activities workerActivities
	Queue      *Queue
	Repo       github.Repo
	RootName   string

	// mutable
	State WorkerState
}

func (w *Worker) Work(ctx workflow.Context) {
	for {
		w.State = WaitingWorkerState

		err := workflow.Await(ctx, func() bool {
			return !w.Queue.IsEmpty()
		})

		if err == workflow.ErrCanceled {
			logger.Info(ctx, "shutting down worker")
			return
		}

		if err != nil {
			logger.Error(ctx, "failed to wait for valid condition, going to retry")
		}

		w.State = WorkingWorkerState

		msg := w.Queue.Pop()

		revision := msg.Revision
		ctx := workflow.WithValue(ctx, config.RevisionLogKey, revision)

		err = w.work(ctx, revision)

		if err != nil {
			logger.Error(ctx, "failed to deploy revision, moving to next one")
		}

		// do all the rest of the work

		// fetch deployments

		// generate deployment id

		// validate things

		// prompt for approval if invalid and lock future deploys if need be
		// we'll need to update the status of everything in the queue somehow
	}
}

func (w *Worker) work(ctx workflow.Context, revision string) error {
	id, err := generateID(ctx)

	ctx = workflow.WithValue(ctx, config.DeploymentIDLogKey, id)

	logger.Info(ctx, "Generated id")

	if err != nil {
		return errors.Wrap(err, "generating id")
	}

	deployedRevision, err := w.fetchLatestDeployment(ctx)

	if err != nil {
		return errors.Wrap(err, "fetching latest deployment")
	}

	logger.Info(ctx, fmt.Sprintf("latest deployed revision %s", deployedRevision))

	return nil
}

func (w *Worker) fetchLatestDeployment(ctx workflow.Context) (string, error) {
	request := activities.FetchLatestDeploymentRequest{
		RepositoryURL: w.Repo.URL,
		RootName:      w.RootName,
	}

	var resp activities.FetchLatestDeploymentResponse
	err := workflow.ExecuteActivity(ctx, w.Activities.FetchLatestDeployment, request).Get(ctx, &resp)

	return resp.Revision, err
}

func generateID(ctx workflow.Context) (uuid.UUID, error) {
	// UUIDErr allows us to extract both the id and the err from the sideeffect
	// not sure if there is a better way to do this
	type UUIDErr struct {
		id  uuid.UUID
		err error
	}

	var result UUIDErr
	encodedResult := workflow.SideEffect(ctx, func(ctx workflow.Context) interface{} {
		uuid, err := uuid.NewUUID()

		return UUIDErr{
			id:  uuid,
			err: err,
		}
	})

	err := encodedResult.Get(&result)

	if err != nil {
		return uuid.UUID{}, errors.Wrap(err, "getting uuid from side effect")
	}

	return result.id, result.err
}
