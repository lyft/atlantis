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
	RemainingApprovals []*github.PullRequestReview
}

func testDismissWorkflow(ctx workflow.Context, r dismissRequest) (dismissResponse, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: time.Minute,
	})
	dismisser := &policy.StaleReviewDismisser{
		GithubActivities: r.GithubActivities,
		PRNumber:         1,
	}
	remainingApprovals, err := dismisser.Dismiss(ctx, r.Revision, r.CurrentApprovals)
	return dismissResponse{
		RemainingApprovals: remainingApprovals,
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
	assert.Empty(t, resp.RemainingApprovals)
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
	assert.Equal(t, resp.RemainingApprovals[0], testApproval)
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
