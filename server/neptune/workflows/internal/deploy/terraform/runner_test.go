package terraform_test

import (
	"context"
	"testing"
	"time"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/root"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/sideeffect"
	terraformWorkflow "github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type testDbActivities struct {
}

func (t *testDbActivities) FetchLatestDeployment(ctx context.Context, request activities.FetchLatestDeploymentRequest) (activities.FetchLatestDeploymentResponse, error) {
	return activities.FetchLatestDeploymentResponse{}, nil
}

func (t *testDbActivities) StoreLatestDeployment(ctx context.Context, request activities.StoreLatestDeploymentRequest) error {
	return nil
}

type testStateReceiver struct {
	payloads []testSignalPayload
}

func (r *testStateReceiver) Receive(ctx workflow.Context, c workflow.ReceiveChannel, deploymentInfo terraform.DeploymentInfo) {

	var payload testSignalPayload
	c.Receive(ctx, &payload)

	r.payloads = append(r.payloads, payload)
}

type testSignalPayload struct {
	S string
}

// signals parent twice with a sleep in between to mimic what our real terraform workflow would be like
func testTerraformWorkflow(ctx workflow.Context, request terraformWorkflow.Request) error {
	info := workflow.GetInfo(ctx)
	parentExecution := info.ParentWorkflowExecution

	payload := testSignalPayload{
		S: "hello",
	}

	if err := workflow.SignalExternalWorkflow(ctx, parentExecution.ID, parentExecution.RunID, state.WorkflowStateChangeSignal, payload).Get(ctx, nil); err != nil {
		return err
	}

	if err := workflow.Sleep(ctx, 5*time.Second); err != nil {
		return err
	}

	return workflow.SignalExternalWorkflow(ctx, parentExecution.ID, parentExecution.RunID, state.WorkflowStateChangeSignal, payload).Get(ctx, nil)
}

type request struct {
}

type response struct {
	Payloads []testSignalPayload
}

func parentWorkflow(ctx workflow.Context, r request) (response, error) {
	receiver := &testStateReceiver{}

	var da *testDbActivities
	runner := &terraform.WorkflowRunner{
		StateReceiver: receiver,
		Repo:          github.Repo{},
		Workflow:      testTerraformWorkflow,
		DbActivities:  da,
	}

	uuid, err := sideeffect.GenerateUUID(ctx)

	if err != nil {
		return response{}, nil
	}

	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Second})
	if err := runner.Run(ctx, terraform.DeploymentInfo{
		ID:         uuid,
		Revision:   "1234",
		CheckRunID: 1,
		Root:       root.Root{},
	}); err != nil {
		return response{}, err
	}

	return response{
		Payloads: receiver.payloads,
	}, nil
}

func TestWorkflowRunner_Run(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	env.RegisterWorkflow(testTerraformWorkflow)

	a := &testDbActivities{}
	env.RegisterActivity(a)

	env.OnActivity(a.FetchLatestDeployment, mock.Anything, mock.Anything).Return(activities.FetchLatestDeploymentResponse{}, nil)
	env.OnActivity(a.StoreLatestDeployment, mock.Anything, mock.Anything).Return(nil)
	env.ExecuteWorkflow(parentWorkflow, request{})

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
