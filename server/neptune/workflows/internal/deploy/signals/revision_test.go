package signals_test

import (
	"testing"
	"time"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision/queue"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/signals"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type testQueue struct {
	Queue []string
}

func (q *testQueue) Push(msg queue.Message) {
	q.Queue = append(q.Queue, msg.Revision)
}

type response struct {
	Queue   []string
	Timeout bool
}

func testWorkflow(ctx workflow.Context) (response, error) {
	var timeout bool
	queue := &testQueue{}

	receiver := signals.NewRevisionSignalReceiver(ctx, queue)
	selector := workflow.NewSelector(ctx)

	receiver.AddReceiveWithTimeout(ctx, selector, 30*time.Second)

	for {
		selector.Select(ctx)

		if receiver.DidTimeout() {
			timeout = true
		}

		if !selector.HasPending() {
			break
		}
	}

	return response{
		Queue:   queue.Queue,
		Timeout: timeout,
	}, nil
}

func TestEnqueue(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	revision := "1234"

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signals.NewRevisionID, signals.NewRevisionRequest{
			Revision: revision,
		})
	}, 0)
	env.ExecuteWorkflow(testWorkflow)
	assert.True(t, env.IsWorkflowCompleted())

	var resp response
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)

	assert.Equal(t, []string{revision}, resp.Queue)
	assert.False(t, resp.Timeout)
}

func TestTimeout(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	// don't send signal to test the timeout
	env.ExecuteWorkflow(testWorkflow)
	assert.True(t, env.IsWorkflowCompleted())

	var resp response
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)

	assert.True(t, resp.Timeout)
}
