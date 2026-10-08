package deploy_test

import (
	"errors"
	"testing"
	"time"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/lock"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision/queue"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/version"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type fakeContinuer struct {
	idle           bool
	snapshotCalled bool
	state          deploy.ContinuedState
}

func (c *fakeContinuer) Idle() bool {
	return c.idle
}

func (c *fakeContinuer) Snapshot(workflow.Context) deploy.ContinuedState {
	c.snapshotCalled = true
	return c.state
}

// blockingWorker waits like a worker on a locked queue until it's shut down.
type blockingWorker struct {
	stopped bool
}

func (w *blockingWorker) GetState() queue.WorkerState {
	return queue.WaitingWorkerState
}

func (w *blockingWorker) Work(ctx workflow.Context) {
	_ = workflow.Await(ctx, func() bool { return false })
	w.stopped = true
}

type continueAsNewRequest struct {
	Idle bool
	// BecomeIdleAfter makes the continuer idle after this long, if set.
	BecomeIdleAfter    time.Duration
	HistoryLengthLimit int
}

var carriedState = deploy.ContinuedState{
	Queue: []terraform.DeploymentInfo{{CheckRunID: 1234}},
	Lock:  lock.LockState{Status: lock.LockedStatus, Revision: "abc123"},
	CheckRuns: map[string]int64{
		"id_atlantis/deploy: root": 1234,
	},
}

func continueAsNewTestWorkflow(ctx workflow.Context, r continueAsNewRequest) error {
	continuer := &fakeContinuer{idle: r.Idle, state: carriedState}
	worker := &blockingWorker{}

	// a non-empty queue keeps the workflow from shutting down on its own,
	// like a locked queue with merged revisions waiting
	q := &testStringContainer{item: "queued"}

	runner := &deploy.Runner{
		Timeout: 10 * time.Second,
		NotifierPeriod: func(ctx workflow.Context, _ int) time.Duration {
			return time.Hour
		},
		Notifier:                   &notifier{},
		Queue:                      q,
		QueueWorker:                worker,
		RevisionReceiver:           &receiver{ctx: ctx},
		NewRevisionSignalChannel:   workflow.GetSignalChannel(ctx, testSignalID),
		Scope:                      metrics.NewNullableScope(),
		Request:                    deploy.Request{Repo: deploy.Repo{FullName: "owner/repo"}, Root: deploy.Root{Name: "root"}},
		Continuer:                  continuer,
		ContinueAsNewSignalChannel: workflow.GetSignalChannel(ctx, deploy.ContinueAsNewSignalID),
		HistoryLengthLimit:         r.HistoryLengthLimit,
	}

	if r.BecomeIdleAfter > 0 {
		workflow.Go(ctx, func(ctx workflow.Context) {
			_ = workflow.Sleep(ctx, r.BecomeIdleAfter)
			continuer.idle = true
		})
	}

	// end the test eventually if the runner never continues as new
	workflow.Go(ctx, func(ctx workflow.Context) {
		_ = workflow.Sleep(ctx, 45*time.Second)
		q.item = ""
	})

	err := runner.Run(ctx)
	if workflow.IsContinueAsNewError(err) {
		if !worker.stopped || !continuer.snapshotCalled {
			return errors.New("continued as new without stopping the worker and taking a snapshot")
		}
	}
	return err
}

func carriedRequest(t *testing.T, err error) deploy.Request {
	var canErr *workflow.ContinueAsNewError
	require.True(t, errors.As(err, &canErr), "expected continue-as-new, got %v", err)
	assert.Equal(t, "continueAsNewTestWorkflow", canErr.WorkflowType.Name)

	var request deploy.Request
	require.NoError(t, converter.GetDefaultDataConverter().FromPayloads(canErr.Input, &request))
	return request
}

func TestRunner_ContinueAsNew(t *testing.T) {
	t.Run("requested while idle", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()
		env.RegisterDelayedCallback(func() {
			env.SignalWorkflow(deploy.ContinueAsNewSignalID, nil)
		}, 5*time.Second)

		env.ExecuteWorkflow(continueAsNewTestWorkflow, continueAsNewRequest{Idle: true, HistoryLengthLimit: deploy.HistoryLengthLimit})

		request := carriedRequest(t, env.GetWorkflowError())
		assert.Equal(t, "owner/repo", request.Repo.FullName)
		assert.Equal(t, "root", request.Root.Name)
		require.NotNil(t, request.ContinuedState)
		assert.Equal(t, carriedState, *request.ContinuedState)
	})

	t.Run("requested while deploying waits for an idle point", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()
		env.RegisterDelayedCallback(func() {
			env.SignalWorkflow(deploy.ContinueAsNewSignalID, nil)
		}, 5*time.Second)

		env.ExecuteWorkflow(continueAsNewTestWorkflow, continueAsNewRequest{BecomeIdleAfter: 15 * time.Second, HistoryLengthLimit: deploy.HistoryLengthLimit})

		// the request arrives at 5s while busy and is honored at the 20s timeout
		request := carriedRequest(t, env.GetWorkflowError())
		assert.Equal(t, carriedState, *request.ContinuedState)
	})

	t.Run("requested but never idle", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()
		env.RegisterDelayedCallback(func() {
			env.SignalWorkflow(deploy.ContinueAsNewSignalID, nil)
		}, 5*time.Second)

		env.ExecuteWorkflow(continueAsNewTestWorkflow, continueAsNewRequest{Idle: false, HistoryLengthLimit: deploy.HistoryLengthLimit})

		assert.NoError(t, env.GetWorkflowError())
	})

	t.Run("history over the limit while idle", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()
		env.SetCurrentHistoryLength(deploy.HistoryLengthLimit + 1)

		env.ExecuteWorkflow(continueAsNewTestWorkflow, continueAsNewRequest{Idle: true, HistoryLengthLimit: deploy.HistoryLengthLimit})

		request := carriedRequest(t, env.GetWorkflowError())
		assert.Equal(t, carriedState, *request.ContinuedState)
	})

	t.Run("history over the limit in a workflow started before this change", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()
		env.SetCurrentHistoryLength(deploy.HistoryLengthLimit + 1)
		env.OnGetVersion(version.ContinueAsNew, workflow.DefaultVersion, workflow.Version(1)).Return(workflow.DefaultVersion)

		env.ExecuteWorkflow(continueAsNewTestWorkflow, continueAsNewRequest{Idle: true, HistoryLengthLimit: deploy.HistoryLengthLimit})

		assert.NoError(t, env.GetWorkflowError())
	})

	t.Run("history under the limit", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()
		env.SetCurrentHistoryLength(deploy.HistoryLengthLimit)

		env.ExecuteWorkflow(continueAsNewTestWorkflow, continueAsNewRequest{Idle: true, HistoryLengthLimit: deploy.HistoryLengthLimit})

		assert.NoError(t, env.GetWorkflowError())
	})

	t.Run("history over the limit while deploying", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()
		env.SetCurrentHistoryLength(deploy.HistoryLengthLimit + 1)

		env.ExecuteWorkflow(continueAsNewTestWorkflow, continueAsNewRequest{Idle: false, HistoryLengthLimit: deploy.HistoryLengthLimit})

		assert.NoError(t, env.GetWorkflowError())
	})
}
