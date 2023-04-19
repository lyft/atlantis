package terraform_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	internalTerraform "github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/terraform"
	terraformWorkflow "github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type testStateReceiver struct {
	payloads []testSignalPayload
}

func (r *testStateReceiver) Receive(ctx workflow.Context, c workflow.ReceiveChannel, info internalTerraform.PRRootInfo) {
	var payload testSignalPayload
	c.Receive(ctx, &payload)

	r.payloads = append(r.payloads, payload)
}

type testSignalPayload struct {
	S string
}

// signals parent twice with a sleep in between to mimic what our real terraform workflow would be like
func testTerraformWorkflow(ctx workflow.Context, request terraformWorkflow.Request) (terraformWorkflow.Response, error) {
	info := workflow.GetInfo(ctx)
	parentExecution := info.ParentWorkflowExecution

	payload := testSignalPayload{
		S: "hello",
	}

	if err := workflow.SignalExternalWorkflow(ctx, parentExecution.ID, parentExecution.RunID, state.WorkflowStateChangeSignal, payload).Get(ctx, nil); err != nil {
		return terraformWorkflow.Response{}, err
	}

	if err := workflow.Sleep(ctx, 5*time.Second); err != nil {
		return terraformWorkflow.Response{}, err
	}

	return terraformWorkflow.Response{}, workflow.SignalExternalWorkflow(ctx, parentExecution.ID, parentExecution.RunID, state.WorkflowStateChangeSignal, payload).Get(ctx, nil)
}

type request struct {
	Info internalTerraform.PRRootInfo
}

type response struct {
	Payloads []testSignalPayload
}

func parentWorkflow(ctx workflow.Context, r request) (response, error) {
	receiver := &testStateReceiver{}
	runner := &internalTerraform.WorkflowRunner{
		StateReceiver: receiver,
		Workflow:      testTerraformWorkflow,
	}

	_, err := runner.Run(ctx, r.Info)
	if err != nil {
		return response{}, err
	}

	return response{
		Payloads: receiver.payloads,
	}, nil
}

func buildPRRootInfo() internalTerraform.PRRootInfo {
	uuid := uuid.New()

	return internalTerraform.PRRootInfo{
		ID: uuid,
		Commit: github.Commit{
			Revision: "1234",
		},
		Root: terraform.Root{
			Name: "some-root",
			Plan: terraform.PlanJob{},
		},
		Repo: github.Repo{},
	}
}

func TestWorkflowRunner_Run(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	env.RegisterWorkflow(testTerraformWorkflow)

	env.ExecuteWorkflow(parentWorkflow, request{
		Info: buildPRRootInfo(),
	})

	var resp response
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)

	assert.Len(t, resp.Payloads, 2)

	for _, p := range resp.Payloads {
		assert.Equal(t, testSignalPayload{
			S: "hello",
		}, p)
	}
}
