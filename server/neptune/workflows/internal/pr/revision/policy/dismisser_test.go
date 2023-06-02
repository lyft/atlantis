package policy_test

import (
	"context"
	"github.com/google/go-github/v45/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	gh "github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision/policy"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
	"testing"
	"time"
)

type dismissRequest struct {
	Revision         revision.Revision
	CurrentApprovals []*github.PullRequestReview
	GithubActivities *mockDismissGHActivities
}

type dismissResponse struct {
	RemainingReviews []*github.PullRequestReview
}

func testDismissWorkflow(ctx workflow.Context, r dismissRequest) (dismissResponse, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: time.Minute,
	})
	dismisser := &policy.StaleReviewDismisser{
		GithubActivities: r.GithubActivities,
		PRNumber:         1,
	}
	teams := map[string][]string{
		"team1": {"user1", "user2"},
		"team2": {"user2", "user3"},
	}
	remainingReviews, err := dismisser.Dismiss(ctx, r.Revision, teams, r.CurrentApprovals)
	return dismissResponse{
		RemainingReviews: remainingReviews,
	}, err
}

func TestStaleReviewDismisser(t *testing.T) {
	testRepo := gh.Repo{Name: "testRepo"}
	oldSHA := "oldSHA"
	oldSHA2 := "oldSHA2"
	testSHA := "testSHA"
	oldTime := time.Now().Add(-time.Hour)
	oldTime2 := time.Now().Add(-time.Hour * 2)
	newTime := time.Now()
	testCommits := activities.ListPRCommitsResponse{
		Commits: []*github.RepositoryCommit{
			{
				SHA:    github.String(oldSHA),
				Commit: &github.Commit{Committer: &github.CommitAuthor{Date: &oldTime}},
			},
			{
				SHA:    github.String(testSHA),
				Commit: &github.Commit{Committer: &github.CommitAuthor{Date: &newTime}},
			},
			{
				SHA:    github.String(oldSHA2),
				Commit: &github.Commit{Committer: &github.CommitAuthor{Date: &oldTime2}},
			},
		},
	}
	testRevision := revision.Revision{
		Repo:     testRepo,
		Revision: testSHA,
	}
	ga := &mockDismissGHActivities{
		Commits: testCommits,
	}
	testApproval := &github.PullRequestReview{
		CommitID: github.String(oldSHA),
		State:    github.String(policy.ApprovalState),
		User:     &github.User{Login: github.String("user2")},
	}
	req := dismissRequest{
		Revision:         testRevision,
		CurrentApprovals: []*github.PullRequestReview{testApproval},
		GithubActivities: ga,
	}
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterActivity(ga)
	env.ExecuteWorkflow(testDismissWorkflow, req)
	var resp dismissResponse
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)
	assert.Empty(t, resp.RemainingReviews)
	assert.True(t, ga.GithubDismissCalled)
}

func TestStaleReviewDismisser_NoStaleCommits(t *testing.T) {
	testRepo := gh.Repo{Name: "testRepo"}
	testSHA := "testSHA"
	newTime := time.Now()
	testCommits := activities.ListPRCommitsResponse{
		Commits: []*github.RepositoryCommit{
			{
				SHA:    github.String(testSHA),
				Commit: &github.Commit{Committer: &github.CommitAuthor{Date: &newTime}},
			},
		},
	}
	testRevision := revision.Revision{
		Repo:     testRepo,
		Revision: testSHA,
	}
	ga := &mockDismissGHActivities{
		Commits: testCommits,
	}
	testApproval := &github.PullRequestReview{
		CommitID: github.String(testSHA),
	}
	req := dismissRequest{
		Revision:         testRevision,
		CurrentApprovals: []*github.PullRequestReview{testApproval},
		GithubActivities: ga,
	}
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterActivity(ga)
	env.ExecuteWorkflow(testDismissWorkflow, req)
	var resp dismissResponse
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)
	assert.Equal(t, resp.RemainingReviews[0], testApproval)
	assert.False(t, ga.GithubDismissCalled)
}

func TestStaleReviewDismisser_InvalidReviews(t *testing.T) {
	testRepo := gh.Repo{Name: "testRepo"}
	testSHA := "testSHA"
	newTime := time.Now()
	testCommits := activities.ListPRCommitsResponse{
		Commits: []*github.RepositoryCommit{
			{
				SHA:    github.String(testSHA),
				Commit: &github.Commit{Committer: &github.CommitAuthor{Date: &newTime}},
			},
		},
	}
	testRevision := revision.Revision{
		Repo:     testRepo,
		Revision: testSHA,
	}
	ga := &mockDismissGHActivities{
		Commits: testCommits,
	}
	testNonApproval := &github.PullRequestReview{
		CommitID: github.String(testSHA),
		State:    github.String("NotApproved"),
		User:     &github.User{Login: github.String("user1")},
	}
	testApprovalFromNonOwner := &github.PullRequestReview{
		CommitID: github.String(testSHA),
		State:    github.String(policy.ApprovalState),
		User:     &github.User{Login: github.String("user10")},
	}
	req := dismissRequest{
		Revision:         testRevision,
		CurrentApprovals: []*github.PullRequestReview{testNonApproval, testApprovalFromNonOwner},
		GithubActivities: ga,
	}
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterActivity(ga)
	env.ExecuteWorkflow(testDismissWorkflow, req)
	var resp dismissResponse
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)
	assert.Equal(t, resp.RemainingReviews[0], testNonApproval)
	assert.Equal(t, resp.RemainingReviews[1], testApprovalFromNonOwner)
	assert.False(t, ga.GithubDismissCalled)
}

func TestStaleReviewDismisser_ListCommitsFailure(t *testing.T) {
	testRepo := gh.Repo{Name: "testRepo"}
	testSHA := "testSHA"
	testRevision := revision.Revision{
		Repo:     testRepo,
		Revision: testSHA,
	}
	ga := &mockDismissGHActivities{
		CommitsErr: true,
	}
	testApproval := &github.PullRequestReview{
		CommitID: github.String(testSHA),
	}
	req := dismissRequest{
		Revision:         testRevision,
		CurrentApprovals: []*github.PullRequestReview{testApproval},
		GithubActivities: ga,
	}
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterActivity(ga)
	env.ExecuteWorkflow(testDismissWorkflow, req)
	var resp dismissResponse
	err := env.GetWorkflowResult(&resp)
	assert.False(t, ga.GithubDismissCalled)
	assert.Error(t, err)
}

func TestStaleReviewDismisser_DismissFailure(t *testing.T) {
	testRepo := gh.Repo{Name: "testRepo"}
	oldSHA := "oldSHA"
	testSHA := "testSHA"
	oldTime := time.Now().Add(-time.Hour)
	newTime := time.Now()
	testCommits := activities.ListPRCommitsResponse{
		Commits: []*github.RepositoryCommit{
			{
				SHA:    github.String(oldSHA),
				Commit: &github.Commit{Committer: &github.CommitAuthor{Date: &oldTime}},
			},
			{
				SHA:    github.String(testSHA),
				Commit: &github.Commit{Committer: &github.CommitAuthor{Date: &newTime}},
			},
		},
	}
	testRevision := revision.Revision{
		Repo:     testRepo,
		Revision: testSHA,
	}
	ga := &mockDismissGHActivities{
		Commits:      testCommits,
		DismissError: true,
	}
	testApproval := &github.PullRequestReview{
		CommitID: github.String(oldSHA),
		State:    github.String(policy.ApprovalState),
		User:     &github.User{Login: github.String("user3")},
	}
	req := dismissRequest{
		Revision:         testRevision,
		CurrentApprovals: []*github.PullRequestReview{testApproval},
		GithubActivities: ga,
	}
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterActivity(ga)
	env.ExecuteWorkflow(testDismissWorkflow, req)
	var resp dismissResponse
	err := env.GetWorkflowResult(&resp)
	assert.True(t, ga.GithubDismissCalled)
	assert.Error(t, err)
}

type mockDismissGHActivities struct {
	GithubDismissCalled bool
	DismissError        bool
	Commits             activities.ListPRCommitsResponse
	CommitsErr          bool
}

func (m *mockDismissGHActivities) GithubListPRCommits(ctx context.Context, request activities.ListPRCommitsRequest) (activities.ListPRCommitsResponse, error) {
	if m.CommitsErr {
		return activities.ListPRCommitsResponse{}, assert.AnError
	}
	return m.Commits, nil
}

func (m *mockDismissGHActivities) GithubDismiss(ctx context.Context, request activities.DismissRequest) (activities.DismissResponse, error) {
	m.GithubDismissCalled = true
	if m.DismissError {
		return activities.DismissResponse{}, assert.AnError
	}
	return activities.DismissResponse{}, nil
}
