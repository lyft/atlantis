package queue

import (
	"fmt"

	"github.com/pkg/errors"
	internalContext "github.com/runatlantis/atlantis/server/neptune/context"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/deployment"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/config/logger"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type revisionProcessor interface {
	Process(ctx workflow.Context, requestedDeployment terraform.DeploymentInfo, latestDeployment *deployment.Info) (*deployment.Info, error)
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
	Queue             *Deploy
	RevisionProcessor revisionProcessor

	// mutable
	state WorkerState
}

type actionType string

const (
	canceled = "canceled"
	process  = "process"
	receive  = "receive"
)

// Work pops work off the queue and if the queue is empty,
// it waits for the queue to be non-empty or a cancelation signal
func (w *Worker) Work(ctx workflow.Context) {
	// set to complete once we return else callers could think we are still working based on the 'working' state.
	defer func() {
		w.state = CompleteWorkerState
	}()

	var latestDeployment *deployment.Info

	selector := workflow.NewSelector(ctx)

	for {
		if w.Queue.IsEmpty() {
			w.state = WaitingWorkerState
		}

		var currentAction actionType
		selector.AddFuture(w.awaitWork(ctx), func(f workflow.Future) {
			err := f.Get(ctx, nil)

			if temporal.IsCanceledError(err) {
				currentAction = canceled
				return
			}

			if err != nil {
				logger.Error(ctx, fmt.Sprintf("Unknown error %s, worker is shutting down", err.Error()))
				currentAction = canceled
				return
			}

			currentAction = process
		})

		var request UnlockSignalRequest
		selector.AddReceive(workflow.GetSignalChannel(ctx, UnlockSignalName), func(c workflow.ReceiveChannel, more bool) {
			_ = c.Receive(ctx, &request)
			currentAction = receive
		})

		switch currentAction {
		case canceled:
			logger.Info(ctx, "Received cancelled signal, worker is shutting down")
			return
		case process:
			d, err := w.process(ctx, latestDeployment)

			if err != nil {
				logger.Error(ctx, "failed to process revision, moving to next one", "err", err)
				continue
			}

			latestDeployment = d
		case receive:
			w.Queue.SetLockStatusForMergedTrigger(UnlockedStatus)
		default:
			logger.Warn(ctx, fmt.Sprintf("%s action not configured. This is probably a bug, skipping for now", currentAction))
		}

	}
}

func (w *Worker) awaitWork(ctx workflow.Context) workflow.Future {
	future, settable := workflow.NewFuture(ctx)

	workflow.Go(ctx, func(ctx workflow.Context) {
		err := workflow.Await(ctx, func() bool {
			return w.Queue.CanPop()
		})

		settable.SetError(err)
	})

	return future
}

func (w *Worker) process(ctx workflow.Context, latestDeployment *deployment.Info) (*deployment.Info, error) {
	w.state = WorkingWorkerState

	msg, err := w.Queue.Pop()

	if err != nil {
		return nil, errors.Wrap(err, "popping off queue")
	}

	ctx = workflow.WithValue(ctx, internalContext.SHAKey, msg.Revision)
	ctx = workflow.WithValue(ctx, internalContext.DeploymentIDKey, msg.ID)
	return w.RevisionProcessor.Process(ctx, msg, latestDeployment)

}

func (w *Worker) GetState() WorkerState {
	return w.state
}
