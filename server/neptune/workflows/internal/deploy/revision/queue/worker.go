package queue

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	internalContext "github.com/runatlantis/atlantis/server/neptune/context"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/deployment"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/config/logger"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type LockStatus string

const (
	UnlockSignalName = "unlock"

	Unlocked = LockStatus("unlocked")
	Locked   = LockStatus("locked")

	LockedRootChecksSummary = "The current root has been locked. This is likely the result of a manual deployment. To override that lock and allow the main branch to perform new deployments, select the Unlock button."
)

func NewLockState() *LockState {
	return &LockState{
		status: Unlocked,
	}
}

type LockState struct {
	status LockStatus
}

func (s *LockState) GetStatus() LockStatus {
	return s.status
}

func (s *LockState) SetStatus(status LockStatus) {
	s.status = status
}

type revisionProcessor interface {
	Process(ctx workflow.Context, requestedDeployment terraform.DeploymentInfo, latestDeployment *deployment.Info) (*deployment.Info, error)
}

type workerActivites interface {
	UpdateCheckRun(ctx context.Context, request activities.UpdateCheckRunRequest) (activities.UpdateCheckRunResponse, error)
}

type WorkerState string

const (
	WaitingWorkerState  WorkerState = "waiting"
	WorkingWorkerState  WorkerState = "working"
	CompleteWorkerState WorkerState = "complete"
)

type UnlockSignalRequest struct {
	User string
}

type Worker struct {
	Queue         *Queue
	Lock          *LockState
	Activities    workerActivites
	ProxySignaler ProxySignaler

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

		ctx := workflow.WithValue(ctx, internalContext.SHAKey, msg.Revision)
		ctx = workflow.WithValue(ctx, internalContext.DeploymentIDKey, msg.ID)

		if w.Lock.GetStatus() == Locked {
			w.waitForUserUnlock(ctx, msg)
		}

		err = w.ProxySignaler.SignalProxyWorkflow(ctx, msg)

		if err != nil {
			logger.Error(ctx, "failed to signal terraform proxy workflow. Redeploy the revision to try again. ", "err", err)
		}
	}
}

func (w *Worker) GetState() WorkerState {
	return w.state
}

// For merged deployments, notify user of a force apply lock status and lock future deployments until signal is received
func (w *Worker) waitForUserUnlock(ctx workflow.Context, msg terraform.DeploymentInfo) error {
	// We won't lock a manually triggered root

	err := workflow.ExecuteActivity(ctx, w.Activities.UpdateCheckRun, activities.UpdateCheckRunRequest{
		Title:   terraform.BuildCheckRunTitle(msg.Root.Name),
		State:   github.CheckRunPending,
		Repo:    msg.Repo,
		ID:      msg.CheckRunID,
		Summary: LockedRootChecksSummary,
		Actions: []github.CheckRunAction{github.CreateUnlockAction()},
	}).Get(ctx, nil)

	if err != nil {
		return errors.Wrap(err, "updating check run")
	}
	// Wait for unlock signal
	signalChan := workflow.GetSignalChannel(ctx, UnlockSignalName)
	var unlockRequest UnlockSignalRequest
	_ = signalChan.Receive(ctx, &unlockRequest)

	w.Lock.SetStatus(Unlocked)
	// TODO: store info on user that unlocked revision, maybe within the check run or just log it?
	return nil
}
