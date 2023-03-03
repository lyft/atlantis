package queue

import (
	"context"
	"testing"
	"time"

	"github.com/runatlantis/atlantis/server/core/config/raw"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func TestDeployer_ShouldRebasePullRequest(t *testing.T) {
	cases := []struct {
		description   string
		root          terraform.Root
		modifiedFiles []string
		shouldReabse  bool
	}{
		{
			description: "default when modified config, root dir modified",
			root: terraform.Root{
				Path:         "test/dir1",
				TrackedFiles: raw.DefaultAutoPlanWhenModified,
			},
			modifiedFiles: []string{"test/dir1/main.tf"},
			shouldReabse:  true,
		},
		{
			description: "default when modified config, root dir not modified",
			root: terraform.Root{
				Path:         "test/dir1",
				TrackedFiles: raw.DefaultAutoPlanWhenModified,
			},
			modifiedFiles: []string{"test/dir2/main.tf"},
			shouldReabse:  false,
		},
		{
			description: "default when modified config, .tfvars file modified",
			root: terraform.Root{
				Path:         "test/dir1",
				TrackedFiles: raw.DefaultAutoPlanWhenModified,
			},
			modifiedFiles: []string{"test/dir1/terraform.tfvars"},
			shouldReabse:  true,
		},
		{
			description: "non default when modified config, non root dir modified",
			root: terraform.Root{
				Path:         "test/dir1",
				TrackedFiles: []string{"**/*.tf*", "../variables.tf"},
			},
			modifiedFiles: []string{"test/variables.tf"},
			shouldReabse:  true,
		},
		{
			description: "non default when modified config, file excluded",
			root: terraform.Root{
				Path:         "test/dir1",
				TrackedFiles: []string{"**/*.tf*", "!exclude.tf"},
			},
			modifiedFiles: []string{"test/dir1/exclude.tf"},
			shouldReabse:  false,
		},
		{
			description: "non default when modified config, file excluded",
			root: terraform.Root{
				Path:         "test/dir1",
				TrackedFiles: []string{"**/*.tf*", "!exclude.tf"},
			},
			modifiedFiles: []string{"test/dir1/exclude.tf"},
			shouldReabse:  false,
		},
		{
			description: "non default when modified config, file excluded and included",
			root: terraform.Root{
				Path:         "test/dir1",
				TrackedFiles: []string{"**/*.tf*", "!exclude.tf"},
			},
			modifiedFiles: []string{"test/dir1/exclude.tf", "test/dir1/main.tf"},
			shouldReabse:  true,
		},
	}

	for _, c := range cases {
		res, err := shouldRebasePullRequest(c.root, c.modifiedFiles)
		assert.NoError(t, err)
		assert.Equal(t, c.shouldReabse, res)
	}
}

type testRebaserActvities struct{}

func (t *testRebaserActvities) GithubListOpenPRs(ctx context.Context, request activities.ListOpenPRsRequest) (activities.ListOpenPRsResponse, error) {
	return activities.ListOpenPRsResponse{}, nil
}

func (t *testRebaserActvities) GithubListModifiedFiles(ctx context.Context, request activities.ListModifiedFilesRequest) (activities.ListModifiedFilesResponse, error) {
	return activities.ListModifiedFilesResponse{}, nil
}

func (t *testRebaserActvities) SetPRRevision(ctx context.Context, request activities.SetPRRevisionRequest) (activities.SetPRRevisionResponse, error) {
	return activities.SetPRRevisionResponse{}, nil
}

type shouldRebaseRequest struct {
	Repo github.Repo
	Root terraform.Root
}

func testShouldRebaseWorkflow(ctx workflow.Context, r shouldRebaseRequest) error {
	options := workflow.ActivityOptions{
		ScheduleToCloseTimeout: 5 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, options)

	var g *testRebaserActvities

	pullRebaser := PullRebaser{
		RebaseActivites: g,
	}

	return pullRebaser.RebaseOpenPRsForRoot(ctx, r.Repo, r.Root, metrics.NewNullableScope())
}

func TestPullRebasePRs_NoOpenPR(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	ga := &testRebaserActvities{}
	env.RegisterActivity(ga)
	repo := github.Repo{
		Owner: "owner",
		Name:  "test",
	}

	root := terraform.Root{
		Name: "test",
	}

	req := shouldRebaseRequest{
		Repo: repo,
		Root: root,
	}

	env.OnActivity(ga.GithubListOpenPRs, mock.Anything, activities.ListOpenPRsRequest{
		Repo: req.Repo,
	}).Return(activities.ListOpenPRsResponse{
		PullRequests: []github.PullRequest{},
	}, nil)

	env.ExecuteWorkflow(testShouldRebaseWorkflow, req)
	env.AssertExpectations(t)

	err := env.GetWorkflowResult(nil)
	assert.Nil(t, err)
}

func TestPullRebasePRs_OpenPR_NeedsRebase(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	ga := &testRebaserActvities{}
	env.RegisterActivity(ga)

	repo := github.Repo{
		Owner: "owner",
		Name:  "test",
	}

	root := terraform.Root{
		Path:         "test/dir2",
		TrackedFiles: raw.DefaultAutoPlanWhenModified,
	}

	req := shouldRebaseRequest{
		Repo: repo,
		Root: root,
	}

	pullRequests := []github.PullRequest{
		{
			Number: 1,
		},
		{
			Number: 2,
		},
	}

	filesModifiedPr1 := []string{"test/dir2/rebase.tf"}
	filesModifiedPr2 := []string{"test/dir1/no-rebase.tf"}

	env.OnActivity(ga.GithubListOpenPRs, mock.Anything, activities.ListOpenPRsRequest{
		Repo: req.Repo,
	}).Return(activities.ListOpenPRsResponse{
		PullRequests: pullRequests,
	}, nil)

	env.OnActivity(ga.GithubListModifiedFiles, mock.Anything, activities.ListModifiedFilesRequest{
		Repo:        repo,
		PullRequest: pullRequests[0],
	}).Return(activities.ListModifiedFilesResponse{
		FilePaths: filesModifiedPr1,
	}, nil)

	env.OnActivity(ga.GithubListModifiedFiles, mock.Anything, activities.ListModifiedFilesRequest{
		Repo:        repo,
		PullRequest: pullRequests[1],
	}).Return(activities.ListModifiedFilesResponse{
		FilePaths: filesModifiedPr2,
	}, nil)

	env.OnActivity(ga.SetPRRevision, mock.Anything, activities.SetPRRevisionRequest{
		Repository:  repo,
		PullRequest: pullRequests[0],
	}).Return(activities.SetPRRevisionResponse{}, nil)

	env.ExecuteWorkflow(testShouldRebaseWorkflow, req)
	env.AssertExpectations(t)

	err := env.GetWorkflowResult(nil)
	assert.Nil(t, err)
}
