package revision_test

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/notifier"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
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

type stateReceiveRequest struct {
	State    *state.Workflow
	RootInfo revision.RootInfo
	T        *testing.T
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
		expectedInfo:  r.RootInfo.ToInternalInfo(),
		expectedState: r.State,
		expectedT:     r.T,
	}

	receiver := &revision.StateReceiver{
		InternalNotifiers: []revision.WorkflowNotifier{
			notifier,
		},
	}

	workflow.Go(ctx, func(ctx workflow.Context) {
		ch.Send(ctx, r.State)
	})

	receiver.Receive(ctx, ch, map[string]revision.RootInfo{r.RootInfo.ID.String(): r.RootInfo})

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

	internalRootInfo := revision.RootInfo{
		ID:   uuid.New(),
		Root: terraform.Root{Name: "root"},
		Repo: github.Repo{Name: "hello"},
		Commit: github.Commit{
			Revision: "12345",
		},
	}

	t.Run("calls notifiers with missing root info", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()

		env.ExecuteWorkflow(testStateReceiveWorkflow, stateReceiveRequest{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.WaitingJobStatus,
				},
				ID: internalRootInfo.ID.String(),
			},
			T: t,
		})

		env.AssertExpectations(t)

		var result stateReceiveResponse
		err = env.GetWorkflowResult(&result)
		assert.False(t, result.NotifierCalled)
		assert.NoError(t, err)
	})

	t.Run("calls notifiers with valid state", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()

		env.ExecuteWorkflow(testStateReceiveWorkflow, stateReceiveRequest{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.WaitingJobStatus,
				},
				ID: internalRootInfo.ID.String(),
			},
			RootInfo: internalRootInfo,
			T:        t,
		})

		env.AssertExpectations(t)

		var result stateReceiveResponse
		err = env.GetWorkflowResult(&result)
		assert.True(t, result.NotifierCalled)
		assert.NoError(t, err)
	})
}
