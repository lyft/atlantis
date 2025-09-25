package pr_test

import (
	"context"
	"testing"
	"time"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type request struct {
	TestPRNumber int
	TestRepo     github.Repo
	ga           *testGithubActivities
}

type response struct {
	ShouldShutdown bool
}

func testWorkflow(ctx workflow.Context, r request) (response, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: time.Minute,
	})
	shutdownChecker := pr.ShutdownStateChecker{
		GithubActivities: r.ga,
		PRNumber:         r.TestPRNumber,
	}
	shouldShutdown := shutdownChecker.ShouldShutdown(ctx, revision.Revision{
		Repo: r.TestRepo,
	})
	return response{ShouldShutdown: shouldShutdown}, nil
}
func TestShutdownStateChecker_ShouldShutdown(t *testing.T) {
	testRepo := github.Repo{
		Owner: "owner",
		Name:  "name",
	}
	testPRNum := 10

	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	ga := &testGithubActivities{
		expectedRepo:     testRepo,
		expectedPRNumber: testPRNum,
		state:            pr.ClosedState,
		t:                t,
	}
	env.RegisterActivity(ga)
	env.ExecuteWorkflow(testWorkflow, request{
		TestPRNumber: testPRNum,
		TestRepo:     testRepo,
		ga:           ga,
	})
	var resp response
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)
	assert.True(t, resp.ShouldShutdown)
}

func TestShutdownStateChecker_ShouldNotShutdown(t *testing.T) {
	testRepo := github.Repo{
		Owner: "owner",
		Name:  "name",
	}
	testPRNum := 10

	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	ga := &testGithubActivities{
		expectedRepo:     testRepo,
		expectedPRNumber: testPRNum,
		state:            "open",
		t:                t,
	}
	env.RegisterActivity(ga)
	env.ExecuteWorkflow(testWorkflow, request{
		TestPRNumber: testPRNum,
		TestRepo:     testRepo,
		ga:           ga,
	})
	var resp response
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)
	assert.False(t, resp.ShouldShutdown)
}

func TestShutdownStateChecker_Error(t *testing.T) {
	testRepo := github.Repo{
		Owner: "owner",
		Name:  "name",
	}
	testPRNum := 10

	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	ga := &testGithubActivities{
		expectedRepo:     testRepo,
		expectedPRNumber: testPRNum,
		state:            pr.ClosedState,
		error:            assert.AnError,
		t:                t,
	}
	env.RegisterActivity(ga)
	env.ExecuteWorkflow(testWorkflow, request{
		TestPRNumber: testPRNum,
		TestRepo:     testRepo,
		ga:           ga,
	})
	var resp response
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)
	assert.False(t, resp.ShouldShutdown)
}

type testGithubActivities struct {
	error            error
	state            string
	t                *testing.T
	expectedPRNumber int
	expectedRepo     github.Repo
}

func (g *testGithubActivities) GithubGetPullRequestState(_ context.Context, request activities.GetPullRequestStateRequest) (activities.GetPullRequestStateResponse, error) {
	assert.Equal(g.t, g.expectedPRNumber, request.PRNumber)
	assert.Equal(g.t, g.expectedRepo, request.Repo)
	return activities.GetPullRequestStateResponse{State: g.state}, g.error
}
