package terraform_test

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/notifier"

	"github.com/google/uuid"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	internalTerraform "github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"github.com/runatlantis/atlantis/server/neptune/workflows/plugins"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type testNotifier struct {
	expectedInfo  notifier.Info
	expectedState *state.Workflow
	expectedT     *testing.T
	called        bool
}

func (n *testNotifier) Notify(ctx workflow.Context, info notifier.Info, s *state.Workflow) error {
	assert.Equal(n.expectedT, n.expectedInfo, info)
	assert.Equal(n.expectedT, n.expectedState, s)

	n.called = true

	return nil
}

type testExternalNotifier struct {
	expectedInfo  plugins.TerraformDeploymentInfo
	expectedState *plugins.TerraformWorkflowState
	expectedT     *testing.T
	called        bool
}

func (n *testExternalNotifier) Notify(ctx workflow.Context, info plugins.TerraformDeploymentInfo, s *plugins.TerraformWorkflowState) error {
	assert.Equal(n.expectedT, n.expectedInfo, info)
	assert.Equal(n.expectedT, n.expectedState, s)

	n.called = true

	return nil
}

type testCheckRunClient struct {
	called bool
}

func (t *testCheckRunClient) CreateOrUpdate(ctx workflow.Context, deploymentID string, request notifier.GithubCheckRunRequest) (int64, error) {
	t.called = true
	return 1, nil
}

type testQueue struct {
	Queue []internalTerraform.DeploymentInfo
}

func (q *testQueue) GetQueuedRevisionsSummary() string {
	var revisions []string
	for _, deploy := range q.Queue {
		revisions = append(revisions, deploy.Commit.Revision)
	}
	return fmt.Sprintf("Revisions in queue: %s", strings.Join(revisions, ", "))
}

func (q *testQueue) GetOrderedMergedItems() []internalTerraform.DeploymentInfo {
	return q.Queue
}

type stateReceiveRequest struct {
	Queue          *testQueue
	CheckRunCache  testCheckRunClient
	State          *state.Workflow
	DeploymentInfo internalTerraform.DeploymentInfo
	T              *testing.T
}

type stateReceiveResponse struct {
	CheckRunCacheCalled    bool
	NotifierCalled         bool
	ExternalNotifierCalled bool
}

func testStateReceiveWorkflow(ctx workflow.Context, r stateReceiveRequest) (stateReceiveResponse, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: 5 * time.Second,
	})
	ch := workflow.NewChannel(ctx)

	notifier := &testNotifier{
		expectedInfo:  r.DeploymentInfo.ToInternalInfo(),
		expectedState: r.State,
		expectedT:     r.T,
	}

	externalNotifier := &testExternalNotifier{
		expectedInfo:  r.DeploymentInfo.ToExternalInfo(),
		expectedState: r.State.ToExternalWorkflowState(),
		expectedT:     r.T,
	}

	receiver := &internalTerraform.StateReceiver{
		Queue:         r.Queue,
		CheckRunCache: &r.CheckRunCache,
		InternalNotifiers: []internalTerraform.WorkflowNotifier{
			notifier,
		},
		AdditionalNotifiers: []plugins.TerraformWorkflowNotifier{
			externalNotifier,
		},
	}

	workflow.Go(ctx, func(ctx workflow.Context) {
		ch.Send(ctx, r.State)
	})

	receiver.Receive(ctx, ch, r.DeploymentInfo)

	return stateReceiveResponse{
		CheckRunCacheCalled:    r.CheckRunCache.called,
		NotifierCalled:         notifier.called,
		ExternalNotifierCalled: externalNotifier.called,
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

	queue := &testQueue{
		Queue: []internalTerraform.DeploymentInfo{
			{
				CheckRunID: 0,
				ID:         uuid.New(),
				Root:       terraform.Root{Name: "root"},
				Repo:       github.Repo{Name: "hello"},
				Commit: github.Commit{
					Revision: "56789",
				},
			},
		},
	}

	t.Run("calls notifiers with state", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()

		env.ExecuteWorkflow(testStateReceiveWorkflow, stateReceiveRequest{
			Queue:         queue,
			CheckRunCache: testCheckRunClient{},
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.WaitingJobStatus,
				},
				Apply: &state.Job{
					Output: jobOutput,
					Status: state.WaitingJobStatus,
					OnWaitingActions: state.JobActions{
						Actions: []state.JobAction{
							{
								ID:   state.ConfirmAction,
								Info: "Confirm this plan to proceed to apply",
							},
							{
								ID:   state.RejectAction,
								Info: "Reject this plan to prevent the apply",
							},
						},
						Summary: "some reason",
					},
				},
			},
			DeploymentInfo: internalDeploymentInfo,
			T:              t,
		})

		env.AssertExpectations(t)

		var result stateReceiveResponse
		err = env.GetWorkflowResult(&result)
		assert.True(t, result.CheckRunCacheCalled)
		assert.True(t, result.NotifierCalled)
		assert.True(t, result.ExternalNotifierCalled)
		assert.NoError(t, err)
	})
}
