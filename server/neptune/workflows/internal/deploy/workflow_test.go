package deploy_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision/queue"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

const testSignalID = "test-signal"

type queueWorker struct {
	state queue.WorkerState
	ctx   workflow.Context
}

func (w *queueWorker) GetState() queue.WorkerState {
	return w.state
}

func (w *queueWorker) Work(ctx workflow.Context) {
	w.state = queue.WorkingWorkerState
	// sleep and then flip to waiting
	_ = workflow.Sleep(ctx, 60*time.Second)

	w.state = queue.WaitingWorkerState

	// do this so we can check for cancellation status
	w.ctx = ctx
}

type testStringContainer struct {
	item string
}

func (t *testStringContainer) IsEmpty() bool {
	return t.item == ""
}

type notifier struct {
	called bool
}

func (n *notifier) Notify(ctx workflow.Context) error {
	n.called = true
	return nil
}

type receiver struct {
	receiveCalled bool
	ctx           workflow.Context
}

func (n *receiver) Receive(c workflow.ReceiveChannel, more bool) {
	var s string
	c.Receive(n.ctx, &s)
	n.receiveCalled = true
}

type response struct {
	WorkerCtxCancelled bool
	ReceiverCalled     bool
	NotifierCalled     bool
}

type request struct {
	WorkerState queue.WorkerState
	QueueItem   string
}

func testWorkflow(ctx workflow.Context, r request) (response, error) {
	receiver := &receiver{ctx: ctx}
	notifier := &notifier{}

	worker := &queueWorker{state: r.WorkerState}

	q := &testStringContainer{item: r.QueueItem}

	runner := &deploy.Runner{
		Timeout: 10 * time.Second,
		NotifierPeriod: func(ctx workflow.Context, _ int) time.Duration {
			return 5 * time.Second
		},
		Notifier:                 notifier,
		Queue:                    q,
		QueueWorker:              worker,
		RevisionReceiver:         receiver,
		NewRevisionSignalChannel: workflow.GetSignalChannel(ctx, testSignalID),
		Scope:                    metrics.NewNullableScope(),
	}

	workflow.Go(ctx, func(ctx workflow.Context) {
		_ = workflow.Sleep(ctx, 20*time.Second)

		// this ensures the workflow ends.
		q.item = ""
	})

	err := runner.Run(ctx)

	return response{
		WorkerCtxCancelled: worker.ctx.Err() == workflow.ErrCanceled,
		ReceiverCalled:     receiver.receiveCalled,
		NotifierCalled:     notifier.called,
	}, err
}

func TestRunner(t *testing.T) {
	t.Run("cancels waiting worker", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()

		// should timeout since we're not sending any signal
		env.ExecuteWorkflow(testWorkflow, request{})

		var resp response
		err := env.GetWorkflowResult(&resp)
		assert.NoError(t, err)
		assert.Equal(t, response{WorkerCtxCancelled: true, NotifierCalled: true}, resp)
	})

	t.Run("doesn't cancel if queue has items", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()

		// should timeout since we're not sending any signal
		env.ExecuteWorkflow(testWorkflow, request{
			QueueItem: "hi",
		})

		var resp response
		err := env.GetWorkflowResult(&resp)
		assert.NoError(t, err)
		assert.Equal(t, response{WorkerCtxCancelled: true, NotifierCalled: true}, resp)
	})

	t.Run("receives signal and then times out", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()

		env.RegisterDelayedCallback(func() {
			env.SignalWorkflow(testSignalID, "")
		}, 11*time.Second)

		// should timeout after sending the first signal
		env.ExecuteWorkflow(testWorkflow, request{})

		var resp response
		err := env.GetWorkflowResult(&resp)
		assert.NoError(t, err)
		assert.Equal(t, response{WorkerCtxCancelled: true, ReceiverCalled: true, NotifierCalled: true}, resp)
	})

	t.Run("receives signal and then times out new version", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()

		env.RegisterDelayedCallback(func() {
			env.SignalWorkflow(testSignalID, "")
		}, 11*time.Second)

		// should timeout after sending the first signal
		env.ExecuteWorkflow(testWorkflow, request{})

		var resp response
		err := env.GetWorkflowResult(&resp)
		assert.NoError(t, err)
		assert.Equal(t, response{WorkerCtxCancelled: true, ReceiverCalled: true, NotifierCalled: true}, resp)
	})
}

// alwaysFailingActivity is registered as a real Temporal activity so it goes through the SDK's
// actual retry engine, rather than just asserting on the ActivityOptions struct's fields.
func alwaysFailingActivity(_ context.Context) error {
	return errors.New("always fails")
}

func retryBoundsTestWorkflow(ctx workflow.Context) error {
	ctx = workflow.WithActivityOptions(ctx, deploy.DeployActivityOptions())
	return workflow.ExecuteActivity(ctx, alwaysFailingActivity).Get(ctx, nil)
}

func TestDeployActivityOptions_BoundsRetries(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	attempts := 0
	env.SetOnActivityStartedListener(func(_ *activity.Info, _ context.Context, _ converter.EncodedValues) {
		attempts++
	})
	env.RegisterActivity(alwaysFailingActivity)
	env.ExecuteWorkflow(retryBoundsTestWorkflow)

	assert.True(t, env.IsWorkflowCompleted())
	err := env.GetWorkflowError()
	assert.Error(t, err, "expected the workflow to eventually fail rather than retry forever")
	assert.Equal(t, 30, attempts, "expected the activity to stop retrying once DeployActivityOptions' MaximumAttempts is reached")
}
