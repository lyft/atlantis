package pr

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/receiver"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
	"testing"
	"time"
)

type request struct {
	mockRevisionProcessor testRevisionProcessor
	scope                 metrics.Scope
	T                     *testing.T
}

type response struct {
	ProcessCount int
}

const (
	revisionID = "revision"
	shutdownID = "shutdown"
)

func testWorkflow(ctx workflow.Context, r request) (response, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: time.Minute,
	})
	mockRevisionProcessor := &r.mockRevisionProcessor
	revisionReceiver := receiver.NewRevisionReceiver(ctx, r.scope)
	shutdownReceiver := receiver.NewShutdownReceiver(ctx, r.scope)
	runner := &Runner{
		RevisionSignalChannel: workflow.GetSignalChannel(ctx, revisionID),
		RevisionReceiver:      &revisionReceiver,
		ShutdownSignalChannel: workflow.GetSignalChannel(ctx, shutdownID),
		ShutdownReceiver:      &shutdownReceiver,
		RevisionProcessor:     mockRevisionProcessor,
		cancel:                func() {},
	}
	err := runner.Run(ctx)
	return response{
		ProcessCount: mockRevisionProcessor.processCalls,
	}, err
}

func TestWorkflowRunner_Run(t *testing.T) {
	req := request{
		mockRevisionProcessor: testRevisionProcessor{},
	}
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(revisionID, receiver.NewTerraformCommitRequest{
			Revision: "abc",
		})
	}, 2*time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(revisionID, receiver.NewTerraformCommitRequest{
			Revision: "def",
		})
	}, 4*time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(shutdownID, receiver.NewShutdownRequest{})
	}, 6*time.Second)
	env.ExecuteWorkflow(testWorkflow, req)
	var resp response
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)
	assert.Equal(t, 2, resp.ProcessCount)
}

type testRevisionProcessor struct {
	processCalls int
}

func (t *testRevisionProcessor) Process(_ workflow.Context, _ receiver.Revision) []activities.PolicySet {
	t.processCalls = t.processCalls + 1
	return []activities.PolicySet{}
}
