package policy_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-github/v45/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	gh "github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision/policy"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type request struct {
	T                   *testing.T
	Revision            revision.Revision
	WorkflowResponses   []terraform.Response
	ListReviewsResponse activities.ListPRReviewsResponse
	ListReviewsErr      error
	DismissResponse     []*github.PullRequestReview
	DismissErr          error
	FilterResponse      []activities.PolicySet
	FilterErr           error
	GithubActivities    *mockGithubActivities
	Roots               map[string]revision.RootInfo
	State               *state.Workflow
}

type response struct {
	DismisserCalls   int
	DismisserReviews []*github.PullRequestReview
	DismisserErr     error
	FilterCalls      int
	FilterPolicies   []activities.PolicySet
	NotifierCalls    int
}

const (
	reviewID = "review"
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
		t:                 r.T,
	}
	notifier := &mockNotifier{
		t:                     r.T,
		expectedRoots:         r.Roots,
		expectedWorkflowState: r.State,
	}
	handler := &policy.FailedPolicyHandler{
		ReviewSignalChannel: workflow.GetSignalChannel(ctx, reviewID),
		Dismisser:           dismisser,
		PolicyFilter:        filter,
		GithubActivities:    r.GithubActivities,
		PRNumber:            1,
		Scope:               metrics.NewNullableScope(),
		Notifier:            notifier,
	}
	handler.Handle(ctx, r.Revision, r.Roots, r.WorkflowResponses)
	_ = workflow.Sleep(ctx, 5*time.Second) //sleep to test notifier called
	return response{
		DismisserCalls:   dismisser.calls,
		DismisserReviews: dismisser.expectedReviews,
		DismisserErr:     dismisser.err,
		FilterCalls:      filter.calls,
		FilterPolicies:   filter.filteredPolicies,
		NotifierCalls:    notifier.calls,
	}, nil
}

func TestFailedPolicyHandlerRunner_NoRoots(t *testing.T) {
	req := request{
		T:                 t,
		Revision:          revision.Revision{Repo: gh.Repo{Name: "repo"}},
		WorkflowResponses: []terraform.Response{},
	}
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(reviewID, policy.NewReviewRequest{})
	}, 2*time.Second)
	env.ExecuteWorkflow(testWorkflow, req)
	var resp response
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)
	assert.Equal(t, resp.DismisserCalls, 0)
	assert.Equal(t, resp.FilterCalls, 0)
}

func TestFailedPolicyHandlerRunner_Handle(t *testing.T) {
	// only testing success case because handler relies on parent context cancellation to terminate
	testApproval := &github.PullRequestReview{
		State: github.String(policy.ApprovalState),
	}
	ga := &mockGithubActivities{
		reviews: activities.ListPRReviewsResponse{Reviews: []*github.PullRequestReview{testApproval}},
	}
	roots := map[string]revision.RootInfo{
		"testRoot": {},
	}
	req := request{
		T:        t,
		Revision: revision.Revision{Repo: gh.Repo{Name: "repo"}, Revision: "sha"},
		WorkflowResponses: []terraform.Response{
			{
				ValidationResults: []activities.ValidationResult{
					{
						Status:    activities.Fail,
						PolicySet: activities.PolicySet{Name: "policy1"},
					},
					{
						Status:    activities.Success,
						PolicySet: activities.PolicySet{Name: "policy2"},
					},
				},
			},
		},
		GithubActivities: ga,
		DismissResponse:  []*github.PullRequestReview{testApproval},
		Roots:            roots,
		State: &state.Workflow{Result: state.WorkflowResult{
			Status: state.CompleteWorkflowStatus,
			Reason: state.ValidationFailedReason,
		}},
	}
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterActivity(ga)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(reviewID, policy.NewReviewRequest{Revision: "stale"})
	}, 2*time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(reviewID, policy.NewReviewRequest{Revision: "sha"})
	}, 2*time.Second)
	env.ExecuteWorkflow(testWorkflow, req)
	var resp response
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)
	assert.Equal(t, 1, resp.DismisserCalls)
	assert.Equal(t, 1, resp.FilterCalls)
	assert.NoError(t, resp.DismisserErr)
	assert.Empty(t, resp.FilterPolicies)
	assert.Equal(t, testApproval, resp.DismisserReviews[0])
	assert.Equal(t, 1, resp.NotifierCalls)
}

type mockDismisser struct {
	calls           int
	expectedReviews []*github.PullRequestReview
	err             error
}

func (d *mockDismisser) Dismiss(ctx workflow.Context, revision revision.Revision, teams map[string][]string, currentApprovals []*github.PullRequestReview) ([]*github.PullRequestReview, error) {
	d.calls++
	return d.expectedReviews, d.err
}

type mockFilter struct {
	calls             int
	expectedApprovals []*github.PullRequestReview
	filteredPolicies  []activities.PolicySet
	t                 *testing.T
}

func (m *mockFilter) Filter(teams map[string][]string, currentApprovals []*github.PullRequestReview, failedPolicies []activities.PolicySet) []activities.PolicySet {
	m.calls++
	assert.Equal(m.t, m.expectedApprovals, currentApprovals)
	return m.filteredPolicies
}

type mockGithubActivities struct {
	called  bool
	reviews activities.ListPRReviewsResponse
	err     error
}

func (g *mockGithubActivities) GithubListTeamMembers(ctx context.Context, request activities.ListTeamMembersRequest) (activities.ListTeamMembersResponse, error) {
	return activities.ListTeamMembersResponse{}, nil
}

func (g *mockGithubActivities) GithubListPRReviews(ctx context.Context, request activities.ListPRReviewsRequest) (activities.ListPRReviewsResponse, error) {
	g.called = true
	return g.reviews, g.err
}

type mockNotifier struct {
	calls                 int
	t                     *testing.T
	expectedWorkflowState *state.Workflow
	expectedRoots         map[string]revision.RootInfo
}

func (m *mockNotifier) Notify(ctx workflow.Context, workflowState *state.Workflow, roots map[string]revision.RootInfo) {
	m.calls++
	assert.Equal(m.t, m.expectedWorkflowState, workflowState)
	assert.Equal(m.t, m.expectedRoots, roots)
}
