package revision_test

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	terraformActivities "github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/notifier"
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
	goodPolicy = "good-policy"
)

type processRevisionRequest struct {
	T              *testing.T
	Revision       revision.Revision
	TFWorkflowFail bool
	Responses      []terraform.Response
}

type processRevisionResponse struct{}

// test 3 roots, two successful, one failure
// have first two roots contain different failing policies
// confirm policies returned are what we expect
func TestProcess(t *testing.T) {
	t.Run("returns expected policy failures", func(t *testing.T) {
		result1 := activities.ValidationResult{
			PolicySet: activities.PolicySet{Name: goodPolicy},
			Status:    activities.Success,
		}
		result2 := activities.ValidationResult{
			PolicySet: activities.PolicySet{Name: badPolicy2},
			Status:    activities.Fail,
		}
		result3 := activities.ValidationResult{
			PolicySet: activities.PolicySet{Name: badPolicy},
			Status:    activities.Fail,
		}
		responses := []terraform.Response{
			{
				ValidationResults: []activities.ValidationResult{result1, result2, result3},
			},
		}
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()
		env.RegisterWorkflow(testTFWorkflow)
		env.ExecuteWorkflow(testProcessRevisionWorkflow, processRevisionRequest{
			T:         t,
			Responses: responses,
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
		TFStateReceiver: &revision.StateReceiver{},
		TFWorkflow:      tfWorkflow,
		PolicyHandler: &testPolicyHandler{
			expectedRevision:  r.Revision,
			expectedResponses: r.Responses,
			t:                 r.T,
		},
		GithubCheckRunCache: &testCheckRunCache{
			t: r.T,
			expectedRequest: notifier.GithubCheckRunRequest{
				Title: "atlantis/plan",
				Sha:   r.Revision.Revision,
				Repo:  r.Revision.Repo,
				State: github.CheckRunSuccess,
				Mode:  terraformActivities.PR,
			},
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
				PolicySet: activities.PolicySet{Name: goodPolicy},
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
	t                 *testing.T
	expectedResponses []terraform.Response
	expectedRevision  revision.Revision
}

func (p *testPolicyHandler) Handle(ctx workflow.Context, revision revision.Revision, roots map[string]revision.RootInfo, responses []terraform.Response) {
	assert.Equal(p.t, p.expectedRevision, revision)
	assert.Equal(p.t, p.expectedResponses, responses)
}

type testCheckRunCache struct {
	expectedRequest notifier.GithubCheckRunRequest
	t               *testing.T
}

func (c *testCheckRunCache) CreateOrUpdate(ctx workflow.Context, id string, request notifier.GithubCheckRunRequest) (int64, error) {
	assert.Equal(c.t, c.expectedRequest.Title, request.Title)
	return 0, nil
}
