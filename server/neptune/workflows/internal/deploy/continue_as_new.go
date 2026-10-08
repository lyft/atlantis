package deploy

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/lock"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision/queue"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/notifier"
	"go.temporal.io/sdk/workflow"
)

const (
	// ContinueAsNewSignalID asks a deploy workflow to continue as new at its
	// next idle point. Workflows started before continue-as-new existed can't
	// pick it up from a version check, because replaying their history fixes
	// the version at its default, so they are moved over with this signal.
	ContinueAsNewSignalID = "continue-as-new"

	// HistoryLengthLimit is the history length past which the workflow
	// continues as new at its next idle point.
	HistoryLengthLimit = 10000
)

// Continuer decides when the deploy workflow can continue as new and captures
// the state to carry over.
type Continuer interface {
	// Idle reports whether no deployment is running and none can start, which
	// is the only point the workflow can continue as new without losing work.
	Idle() bool
	// Snapshot handles any signals that haven't been processed yet and returns
	// the state to carry into the next run. The queue worker must be stopped.
	Snapshot(ctx workflow.Context) ContinuedState
}

type deployQueue interface {
	CanPop() bool
	Scan() []terraform.DeploymentInfo
	GetLockState() lock.LockState
	SetLockForMergedItems(ctx workflow.Context, state lock.LockState)
}

type queueContinuer struct {
	queue    deployQueue
	worker   interface{ IsDeploying() bool }
	receiver interface {
		ReceiveAsync(workflow.ReceiveChannel) bool
	}
	checkRunCache            *notifier.GithubCheckRunCache
	newRevisionSignalChannel workflow.ReceiveChannel
	unlockSignalChannel      workflow.ReceiveChannel
}

func (c *queueContinuer) Idle() bool {
	return !c.worker.IsDeploying() && !c.queue.CanPop()
}

func (c *queueContinuer) Snapshot(ctx workflow.Context) ContinuedState {
	// Revisions received now are queued as usual, so they're carried over.
	for c.receiver.ReceiveAsync(c.newRevisionSignalChannel) {
	}

	// The stopped worker no longer reads unlock signals, so apply them here.
	var unlock queue.UnlockSignalRequest
	for c.unlockSignalChannel.ReceiveAsync(&unlock) {
		c.queue.SetLockForMergedItems(ctx, lock.LockState{Status: lock.UnlockedStatus})
	}

	return ContinuedState{
		Queue:     c.queue.Scan(),
		Lock:      c.queue.GetLockState(),
		CheckRuns: c.checkRunCache.Entries(),
	}
}

// restoreQueue puts carried-over queue items and check run IDs back before the
// worker starts.
func restoreQueue(state *ContinuedState, q *queue.Deploy, checkRunCache *notifier.GithubCheckRunCache) {
	if state == nil {
		return
	}
	checkRunCache.Restore(state.CheckRuns)
	for _, d := range state.Queue {
		q.Push(d)
	}
}
