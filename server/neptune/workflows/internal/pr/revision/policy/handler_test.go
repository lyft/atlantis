package policy_test

import (
	"context"
	"github.com/google/go-github/v45/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	gh "github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision/policy"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
	"testing"
	"time"
)

type request struct {
	T                     *testing.T
	Revision              revision.Revision
	WorkflowResponses     []terraform.Response
	ListApprovalsResponse activities.ListPRApprovalsResponse
	ListApprovalsErr      error
	DismissResponse       []*github.PullRequestReview
	DismissErr            error
	FilterResponse        []activities.PolicySet
	FilterErr             error
	GithubActivities      *mockGithubActivities
}

type response struct {
	DismisserCalled  bool
	DismisserReviews []*github.PullRequestReview
	DismisserErr     error
	FilterCalled     bool
	FilterErr        error
	FilterPolicies   []activities.PolicySet
}

const (
	approveID = "approve"
)

func testWorkflow(ctx workflow.Context, r request) (response, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: time.Minute,
	})
	dismisser := &mockDismisser{
		expectedReviews: r.DismissResponse,
		err:             r.DismissErr,
	}
	filter := &mockFilter{
		expectedApprovals: r.DismissResponse,
		filteredPolicies:  r.FilterResponse,
		err:               r.FilterErr,
		t:                 r.T,
	}
	handler := &policy.FailedPolicyHandler{
		ApprovalSignalChannel: workflow.GetSignalChannel(ctx, approveID),
		Dismisser:             dismisser,
		PolicyFilter:          filter,
		GithubActivities:      r.GithubActivities,
		PRNumber:              1,
	}
	handler.Handle(ctx, r.Revision, r.WorkflowResponses)
	return response{
		DismisserCalled:  dismisser.called,
		DismisserReviews: dismisser.expectedReviews,
		DismisserErr:     dismisser.err,
		FilterCalled:     filter.called,
		FilterErr:        filter.err,
		FilterPolicies:   filter.filteredPolicies,
	}, nil
}

func TestFailedPolicyHandlerRunner_NoRoots(t *testing.T) {
	req := request{
		T:        t,
		Revision: revision.Revision{Repo: gh.Repo{Name: "repo"}},
		WorkflowResponses: []terraform.Response{
			{
				ValidationResults: []activities.ValidationResult{
					{
						Status:    activities.Success,
						PolicySet: activities.PolicySet{Name: "policy1"},
					},
				},
			},
		},
	}
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(approveID, policy.NewApproveSignalRequest{})
	}, 2*time.Second)
	env.ExecuteWorkflow(testWorkflow, req)
	var resp response
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)
	assert.False(t, resp.DismisserCalled)
	assert.False(t, resp.FilterCalled)
}

func TestFailedPolicyHandlerRunner_Handle(t *testing.T) {
	// only testing success case because handler relies on parent context cancellation to terminate
	testApproval := &github.PullRequestReview{
		State: github.String("APPROVED"),
	}
	ga := &mockGithubActivities{
		approvals: activities.ListPRApprovalsResponse{Approvals: []*github.PullRequestReview{testApproval}},
	}
	req := request{
		T:        t,
		Revision: revision.Revision{Repo: gh.Repo{Name: "repo"}},
		WorkflowResponses: []terraform.Response{
			{
				ValidationResults: []activities.ValidationResult{
					{
						Status:    activities.Fail,
						PolicySet: activities.PolicySet{Name: "policy1"},
					},
				},
			},
		},
		GithubActivities: ga,
		DismissResponse:  []*github.PullRequestReview{testApproval},
	}
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterActivity(ga)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(approveID, policy.NewApproveSignalRequest{})
	}, 2*time.Second)
	env.ExecuteWorkflow(testWorkflow, req)
	var resp response
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)
	assert.True(t, resp.DismisserCalled)
	assert.True(t, resp.FilterCalled)
	assert.NoError(t, resp.DismisserErr)
	assert.NoError(t, resp.FilterErr)
	assert.Empty(t, resp.FilterPolicies)
	assert.Equal(t, resp.DismisserReviews[0], testApproval)
}

type mockDismisser struct {
	called          bool
	expectedReviews []*github.PullRequestReview
	err             error
}

func (d *mockDismisser) Dismiss(ctx workflow.Context, revision revision.Revision, currentApprovals []*github.PullRequestReview) ([]*github.PullRequestReview, error) {
	d.called = true
	return d.expectedReviews, d.err
}

type mockFilter struct {
	called            bool
	expectedApprovals []*github.PullRequestReview
	filteredPolicies  []activities.PolicySet
	err               error
	t                 *testing.T
}

func (m *mockFilter) Filter(ctx workflow.Context, revision revision.Revision, currentApprovals []*github.PullRequestReview, failedPolicies []activities.PolicySet) ([]activities.PolicySet, error) {
	m.called = true
	assert.Equal(m.t, m.expectedApprovals, currentApprovals)
	return m.filteredPolicies, m.err
}

type mockGithubActivities struct {
	called    bool
	approvals activities.ListPRApprovalsResponse
	err       error
}

func (g *mockGithubActivities) GithubListPRApprovals(ctx context.Context, request activities.ListPRApprovalsRequest) (activities.ListPRApprovalsResponse, error) {
	g.called = true
	return g.approvals, g.err
}
