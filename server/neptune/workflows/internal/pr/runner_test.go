package pr

import (
	"context"
	"testing"
	"time"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type request struct {
	mockRevisionProcessor testRevisionProcessor
	scope                 metrics.Scope
	InactivityTimeout     time.Duration
	ShutdownPollTime      time.Duration
	NumShutdownPollTicks  int
	T                     *testing.T
	GithubActivities      *testActivities
}

type response struct {
	ProcessCount int
}

const (
	revisionID = "revision"
	shutdownID = "shutdown"
)

type testActivities struct{}

func (a *testActivities) GithubCreateComment(ctx context.Context, request activities.CreateCommentRequest) (activities.CreateCommentResponse, error) {
	return activities.CreateCommentResponse{}, nil
}

func testWorkflow(ctx workflow.Context, r request) (response, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: time.Minute,
	})
	mockRevisionProcessor := &r.mockRevisionProcessor
	mockShutdownChecker := &testShutdownChecker{
		ShouldShutdownAfterNTicks: r.NumShutdownPollTicks,
	}
	revisionReceiver := revision.NewRevisionReceiver(ctx, r.scope)
	runner := &Runner{
		RevisionSignalChannel: workflow.GetSignalChannel(ctx, revisionID),
		RevisionReceiver:      &revisionReceiver,
		ShutdownSignalChannel: workflow.GetSignalChannel(ctx, shutdownID),
		RevisionProcessor:     mockRevisionProcessor,
		ShutdownChecker:       mockShutdownChecker,
		InactivityTimeout:     r.InactivityTimeout,
		ShutdownPollTick:      r.ShutdownPollTime,
		Scope:                 metrics.NewNullableScope(),
		GithubActivities:      r.GithubActivities,
		PRNumber:              1,
	}
	err := runner.Run(ctx)
	return response{
		ProcessCount: mockRevisionProcessor.processCalls,
	}, err
}

func TestWorkflowRunner_Run(t *testing.T) {
	req := request{
		mockRevisionProcessor: testRevisionProcessor{},
		InactivityTimeout:     time.Minute,
		ShutdownPollTime:      time.Hour,
		NumShutdownPollTicks:  1,
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
	a := &testActivities{}
	req := request{
		mockRevisionProcessor: testRevisionProcessor{},
		InactivityTimeout:     time.Second,
		ShutdownPollTime:      time.Hour,
		GithubActivities:      a,
	}
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterActivity(a)
	commentRequest := activities.CreateCommentRequest{
		PRNumber:    1,
		CommentBody: TimeoutComment,
	}
	env.OnActivity(a.GithubCreateComment, mock.Anything, commentRequest).Return(activities.CompareCommitResponse{}, nil)
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

func TestWorkflowRunner_Run_ShutdownPoll(t *testing.T) {
	req := request{
		mockRevisionProcessor: testRevisionProcessor{},
		InactivityTimeout:     time.Minute,
		ShutdownPollTime:      time.Second,
		NumShutdownPollTicks:  3,
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

type testShutdownChecker struct {
	ShouldShutdownAfterNTicks int
	calls                     int
}

func (c *testShutdownChecker) ShouldShutdown(ctx workflow.Context, prRevision revision.Revision) bool {
	c.calls++
	return c.calls == c.ShouldShutdownAfterNTicks
}
