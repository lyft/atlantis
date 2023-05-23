package pr

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
	"testing"
	"time"
)

type request struct {
	mockRevisionProcessor testRevisionProcessor
	scope                 metrics.Scope
	inactivityTimeout     time.Duration
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
	revisionReceiver := revision.NewRevisionReceiver(ctx, r.scope)
	runner := &Runner{
		RevisionSignalChannel: workflow.GetSignalChannel(ctx, revisionID),
		RevisionReceiver:      &revisionReceiver,
		ShutdownSignalChannel: workflow.GetSignalChannel(ctx, shutdownID),
		RevisionProcessor:     mockRevisionProcessor,
		InactivityTimeout:     r.inactivityTimeout,
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
		inactivityTimeout:     time.Minute,
	}
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(revisionID, revision.NewTerraformRevisionRequest{
			Revision: "abc",
		})
	}, 2*time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(revisionID, revision.NewTerraformRevisionRequest{
			Revision: "def",
		})
	}, 4*time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(shutdownID, NewShutdownRequest{})
	}, 6*time.Second)
	env.ExecuteWorkflow(testWorkflow, req)
	var resp response
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)
	assert.Equal(t, 2, resp.ProcessCount)
}

func TestWorkflowRunner_Run_InactivityTimeout(t *testing.T) {
	req := request{
		mockRevisionProcessor: testRevisionProcessor{},
		inactivityTimeout:     time.Second,
	}
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(revisionID, revision.NewTerraformRevisionRequest{
			Revision: "abc",
		})
	}, 20*time.Second)
	env.ExecuteWorkflow(testWorkflow, req)
	var resp response
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.ProcessCount)
}

type testRevisionProcessor struct {
	processCalls int
}

func (t *testRevisionProcessor) Process(_ workflow.Context, _ revision.Revision) {
	t.processCalls = t.processCalls + 1
}
