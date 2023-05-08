package revision_test

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	terraformActivities "github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
	"testing"
	"time"
)

const (
	badPolicy  = "bad-policy"
	badPolicy2 = "bad-policy-2"
)

type processRevisionRequest struct {
	T                      *testing.T
	Revision               revision.Revision
	TFWorkflowFail         bool
	ExpectedFailedPolicies []activities.PolicySet
}

type processRevisionResponse struct{}

// test 3 roots, two successful, one failure
// have first two roots contain different failing policies
// confirm policies returned are what we expect

func TestProcess(t *testing.T) {
	t.Run("returns expected policy failures", func(t *testing.T) {
		expectedPolicies := []activities.PolicySet{
			{Name: badPolicy},
			{Name: badPolicy2},
		}
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()
		env.RegisterWorkflow(testTFWorkflow)
		env.ExecuteWorkflow(testProcessRevisionWorkflow, processRevisionRequest{
			T:                      t,
			ExpectedFailedPolicies: expectedPolicies,
			Revision: revision.Revision{
				Repo: github.Repo{},
				Roots: []terraformActivities.Root{
					{Name: "some-root"},
				},
			},
		})
		env.AssertExpectations(t)

		var result processRevisionResponse
		err := env.GetWorkflowResult(&result)
		assert.NoError(t, err)
	})
	t.Run("failing child workflow", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()
		env.RegisterWorkflow(testTFWorkflowFailure)
		env.ExecuteWorkflow(testProcessRevisionWorkflow, processRevisionRequest{
			T:              t,
			TFWorkflowFail: true,
			Revision: revision.Revision{
				Repo: github.Repo{},
				Roots: []terraformActivities.Root{
					{Name: "some-root"},
				},
			},
		})
		env.AssertExpectations(t)

		var result processRevisionResponse
		err := env.GetWorkflowResult(&result)
		assert.NoError(t, err)
	})
}

func testProcessRevisionWorkflow(ctx workflow.Context, r processRevisionRequest) (processRevisionResponse, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: 5 * time.Second,
	})

	tfWorkflow := testTFWorkflow
	if r.TFWorkflowFail {
		tfWorkflow = testTFWorkflowFailure
	}
	processor := revision.Processor{
		TFStateReceiver: &revision.StateReceiver{RootCache: make(map[string]revision.RootInfo)},
		TFWorkflow:      tfWorkflow,
		PolicyHandler: testPolicyHandler{
			expectedFailedPolicies: r.ExpectedFailedPolicies,
			t:                      r.T,
		},
	}
	processor.Process(ctx, r.Revision)

	return processRevisionResponse{}, nil
}

func testTFWorkflow(_ workflow.Context, _ terraform.Request) (terraform.Response, error) {
	return terraform.Response{
		ValidationResults: []activities.ValidationResult{
			{
				Status:    activities.Success,
				PolicySet: activities.PolicySet{Name: "good-policy"},
			},
			{
				Status:    activities.Fail,
				PolicySet: activities.PolicySet{Name: badPolicy},
			},
			{
				Status:    activities.Fail,
				PolicySet: activities.PolicySet{Name: badPolicy2},
			},
		},
	}, nil
}

func testTFWorkflowFailure(_ workflow.Context, _ terraform.Request) (terraform.Response, error) {
	return terraform.Response{}, assert.AnError
}

type testPolicyHandler struct {
	t                      *testing.T
	expectedFailedPolicies []activities.PolicySet
}

func (p testPolicyHandler) Process(_ workflow.Context, failedPolicies []activities.PolicySet) {
	assert.Equal(p.t, p.expectedFailedPolicies, failedPolicies)
}
