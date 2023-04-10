package terraform_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	internalTerraform "github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type testNotifier struct {
	expectedInfo  internalTerraform.DeploymentInfo
	expectedState *state.Workflow
	expectedT     *testing.T
	called        bool
}

func (n *testNotifier) Notify(ctx workflow.Context, info internalTerraform.DeploymentInfo, s *state.Workflow) error {
	assert.Equal(n.expectedT, n.expectedInfo, info)
	assert.Equal(n.expectedT, n.expectedState, s)

	n.called = true

	return nil
}

type stateReceiveRequest struct {
	State          *state.Workflow
	DeploymentInfo internalTerraform.DeploymentInfo
	T              *testing.T
}

type stateReceiveResponse struct {
	NotifierCalled bool
}

func testStateReceiveWorkflow(ctx workflow.Context, r stateReceiveRequest) (stateReceiveResponse, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: 5 * time.Second,
	})
	ch := workflow.NewChannel(ctx)

	notifier := &testNotifier{
		expectedInfo:  r.DeploymentInfo,
		expectedState: r.State,
		expectedT:     r.T,
	}

	receiver := &internalTerraform.StateReceiver{
		Notifiers: []internalTerraform.WorkflowNotifier{
			notifier,
		},
	}

	workflow.Go(ctx, func(ctx workflow.Context) {
		ch.Send(ctx, r.State)
	})

	receiver.Receive(ctx, ch, r.DeploymentInfo)

	return stateReceiveResponse{
		NotifierCalled: notifier.called,
	}, nil
}

func TestStateReceive(t *testing.T) {
	outputURL, err := url.Parse("www.nish.com")
	assert.NoError(t, err)

	jobOutput := &state.JobOutput{
		URL: outputURL,
	}

	internalDeploymentInfo := internalTerraform.DeploymentInfo{
		CheckRunID: 1,
		ID:         uuid.New(),
		Root:       terraform.Root{Name: "root"},
		Repo:       github.Repo{Name: "hello"},
		Commit: github.Commit{
			Revision: "12345",
		},
	}

	t.Run("calls notifier with state", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()

		env.ExecuteWorkflow(testStateReceiveWorkflow, stateReceiveRequest{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.WaitingJobStatus,
				},
			},
			DeploymentInfo: internalDeploymentInfo,
			T:              t,
		})

		env.AssertExpectations(t)

		var result stateReceiveResponse
		err = env.GetWorkflowResult(&result)
		assert.True(t, result.NotifierCalled)
		assert.NoError(t, err)
	})
}
