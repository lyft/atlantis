package deploy

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/lock"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision/queue"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/notifier"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

const testRevisionSignal = "test-revision"

func deployment(revision string) terraform.DeploymentInfo {
	return terraform.DeploymentInfo{
		ID:     uuid.NewSHA1(uuid.Nil, []byte(revision)),
		Commit: github.Commit{Revision: revision},
	}
}

type notDeploying struct{}

func (notDeploying) IsDeploying() bool { return false }

// pushingReceiver queues each revision signal, like the real receiver does.
type pushingReceiver struct {
	queue *queue.Deploy
}

func (r *pushingReceiver) ReceiveAsync(c workflow.ReceiveChannel) bool {
	var revision string
	if !c.ReceiveAsync(&revision) {
		return false
	}
	r.queue.Push(deployment(revision))
	return true
}

type snapshotResult struct {
	IdleBefore bool
	State      ContinuedState
}

func snapshotWorkflow(ctx workflow.Context) (snapshotResult, error) {
	q := queue.NewQueue(func(workflow.Context, *queue.Deploy) {}, metrics.NewNullableScope())
	q.Push(deployment("queued"))
	q.SetLockForMergedItems(ctx, lock.LockState{Status: lock.LockedStatus, Revision: "manual"})

	checkRunCache := notifier.NewGithubCheckRunCache(nil)
	checkRunCache.Restore(map[string]int64{"key": 1})

	c := &queueContinuer{
		queue:                    q,
		worker:                   notDeploying{},
		receiver:                 &pushingReceiver{queue: q},
		checkRunCache:            checkRunCache,
		newRevisionSignalChannel: workflow.GetSignalChannel(ctx, testRevisionSignal),
		unlockSignalChannel:      workflow.GetSignalChannel(ctx, queue.UnlockSignalName),
	}

	// let the test's signals arrive before taking the snapshot
	_ = workflow.Sleep(ctx, 5*time.Second)

	return snapshotResult{IdleBefore: c.Idle(), State: c.Snapshot(ctx)}, nil
}

func TestQueueContinuer_Snapshot(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(testRevisionSignal, "merged-later")
		env.SignalWorkflow(queue.UnlockSignalName, queue.UnlockSignalRequest{User: "someone"})
	}, time.Second)

	env.ExecuteWorkflow(snapshotWorkflow)
	require.NoError(t, env.GetWorkflowError())

	var result snapshotResult
	require.NoError(t, env.GetWorkflowResult(&result))

	// a locked queue with items waiting is idle: nothing can be popped
	assert.True(t, result.IdleBefore)
	assert.Equal(t, ContinuedState{
		Queue:     []terraform.DeploymentInfo{deployment("queued"), deployment("merged-later")},
		Lock:      lock.LockState{Status: lock.UnlockedStatus},
		CheckRuns: map[string]int64{"key": 1},
	}, result.State)
}

func TestRestoreQueue(t *testing.T) {
	q := queue.NewQueue(func(workflow.Context, *queue.Deploy) {}, metrics.NewNullableScope())
	checkRunCache := notifier.NewGithubCheckRunCache(nil)

	restoreQueue(&ContinuedState{
		Queue:     []terraform.DeploymentInfo{deployment("first"), deployment("second")},
		CheckRuns: map[string]int64{"key": 7},
	}, q, checkRunCache)

	assert.Equal(t, []terraform.DeploymentInfo{deployment("first"), deployment("second")}, q.Scan())
	assert.Equal(t, map[string]int64{"key": 7}, checkRunCache.Entries())
}

func TestRestoreQueue_noState(t *testing.T) {
	q := queue.NewQueue(func(workflow.Context, *queue.Deploy) {}, metrics.NewNullableScope())
	checkRunCache := notifier.NewGithubCheckRunCache(nil)

	restoreQueue(nil, q, checkRunCache)

	assert.True(t, q.IsEmpty())
	assert.Empty(t, checkRunCache.Entries())
}
