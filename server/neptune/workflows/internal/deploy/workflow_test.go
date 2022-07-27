package deploy_test

import (
	"testing"
	"time"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision/queue"
	"github.com/stretchr/testify/assert"
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

	// do this so we can check for cancellation status
	w.ctx = ctx
	return
}

func (w *queueWorker) addCallback(ctx workflow.Context, selector workflow.Selector) {
	// adding a signal receiver allows us to toggle the queueworker state from our tests
	ch := workflow.GetSignalChannel(ctx, testSignalID)
	selector.AddReceive(ch, func(c workflow.ReceiveChannel, more bool) {
		var state queue.WorkerState
		c.Receive(ctx, &state)

		w.state = state
	})
}

type receiver struct {
	timeoutValue bool
}

func (r *receiver) DidTimeout() bool {
	return r.timeoutValue
}

func (r *receiver) AddTimeout(ctx workflow.Context, selector workflow.Selector) {
	r.timeoutValue = false
	selector.AddFuture(workflow.NewTimer(ctx, 5*time.Second), func(f workflow.Future) {
		r.timeoutValue = true
	})
}

type response struct {
	WorkerCtxCancelled bool
}

type request struct {
	WorkerState queue.WorkerState
}

func testWorkflow(ctx workflow.Context, r request) (response, error) {
	selector := workflow.NewSelector(ctx)
	receiver := &receiver{}
	receiver.AddTimeout(ctx, selector)

	worker := &queueWorker{state: r.WorkerState}
	worker.addCallback(ctx, selector)

	runner := &deploy.Runner{
		QueueWorker:      worker,
		RevisionReceiver: receiver,
		Selector:         selector,
	}

	err := runner.Run(ctx)

	return response{
		WorkerCtxCancelled: worker.ctx.Err() == workflow.ErrCanceled,
	}, err
}

func TestRunner(t *testing.T) {
	t.Run("cancels waiting worker", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()

		env.ExecuteWorkflow(testWorkflow, request{WorkerState: queue.WaitingWorkerState})

		var resp response
		err := env.GetWorkflowResult(&resp)
		assert.NoError(t, err)
		assert.Equal(t, response{WorkerCtxCancelled: true}, resp)
	})

	t.Run("does not cancel working worker", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()

		// send a signal after the first timer fires at 5 seconds to make sure that we're not
		// exiting when the worker is working
		env.RegisterDelayedCallback(func() {
			env.SignalWorkflow(testSignalID, queue.WaitingWorkerState)
		}, 7*time.Second)

		env.ExecuteWorkflow(testWorkflow, request{WorkerState: queue.WorkingWorkerState})
		var resp response
		err := env.GetWorkflowResult(&resp)
		assert.NoError(t, err)
		assert.Equal(t, response{WorkerCtxCancelled: true}, resp)
	})

}
