package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/runatlantis/atlantis/server/core/config/raw"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
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
				WhenModified: raw.DefaultAutoPlanWhenModified,
			},
			modifiedFiles: []string{"test/dir1/main.tf"},
			shouldReabse:  true,
		},
		{
			description: "default when modified config, root dir not modified",
			root: terraform.Root{
				Path:         "test/dir1",
				WhenModified: raw.DefaultAutoPlanWhenModified,
			},
			modifiedFiles: []string{"test/dir2/main.tf"},
			shouldReabse:  false,
		},
		{
			description: "default when modified config, .tfvars file modified",
			root: terraform.Root{
				Path:         "test/dir1",
				WhenModified: raw.DefaultAutoPlanWhenModified,
			},
			modifiedFiles: []string{"test/dir1/terraform.tfvars"},
			shouldReabse:  true,
		},
		{
			description: "non default when modified config, non root dir modified",
			root: terraform.Root{
				Path:         "test/dir1",
				WhenModified: []string{"**/*.tf*", "../variables.tf"},
			},
			modifiedFiles: []string{"test/variables.tf"},
			shouldReabse:  true,
		},
		{
			description: "non default when modified config, file excluded",
			root: terraform.Root{
				Path:         "test/dir1",
				WhenModified: []string{"**/*.tf*", "!exclude.tf"},
			},
			modifiedFiles: []string{"test/dir1/exclude.tf"},
			shouldReabse:  false,
		},
		{
			description: "non default when modified config, file excluded",
			root: terraform.Root{
				Path:         "test/dir1",
				WhenModified: []string{"**/*.tf*", "!exclude.tf"},
			},
			modifiedFiles: []string{"test/dir1/exclude.tf"},
			shouldReabse:  false,
		},
		{
			description: "non default when modified config, file excluded and included",
			root: terraform.Root{
				Path:         "test/dir1",
				WhenModified: []string{"**/*.tf*", "!exclude.tf"},
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

/*
- no open PRs
- rebase and no rebase PR
- rebase PR if list files error out
- rebase PR if file match errors out
*/

type testGithubRebaseActivities struct{}

func (t *testGithubRebaseActivities) GithubListOpenPRs(ctx context.Context, request activities.ListOpenPRsRequest) (activities.ListOpenPRsResponse, error) {
	return activities.ListOpenPRsResponse{}, nil
}

func (t *testGithubRebaseActivities) GithubListModifiedFiles(ctx context.Context, request activities.ListModifiedFilesRequest) (activities.ListModifiedFilesResponse, error) {
	return activities.ListModifiedFilesResponse{}, nil
}

type testBuildNotifyActivities struct{}

func (t *testBuildNotifyActivities) BuildNotifyRebasePR(ctx context.Context, request activities.BuildNotifyRebasePRRequest) (activities.BuildNotifyRebasePRResponse, error) {
	return activities.BuildNotifyRebasePRResponse{}, nil
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

	var g *testGithubRebaseActivities
	var b *testBuildNotifyActivities

	pullRebaser := PullRebaser{
		GithubActivities:      g,
		BuildNotifyActivities: b,
	}

	return pullRebaser.RebaseOpenPRsForRoot(ctx, r.Repo, r.Root)
}

func TestPullRebasePRs_NoOpenPR(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	ga := &testGithubRebaseActivities{}
	ba := &testBuildNotifyActivities{}
	env.RegisterActivity(ga)
	env.RegisterActivity(ba)

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

	ga := &testGithubRebaseActivities{}
	ba := &testBuildNotifyActivities{}
	env.RegisterActivity(ga)
	env.RegisterActivity(ba)

	repo := github.Repo{
		Owner: "owner",
		Name:  "test",
	}

	root := terraform.Root{
		Path:         "test/dir2",
		WhenModified: raw.DefaultAutoPlanWhenModified,
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

	env.OnActivity(ba.BuildNotifyRebasePR, mock.Anything, activities.BuildNotifyRebasePRRequest{
		Repository:  repo,
		PullRequest: pullRequests[0],
	}).Return(activities.BuildNotifyRebasePRResponse{}, nil)

	env.ExecuteWorkflow(testShouldRebaseWorkflow, req)
	env.AssertExpectations(t)

	err := env.GetWorkflowResult(nil)
	assert.Nil(t, err)
}

func TestPullRebasePRs_ListFileError_NeedRebase(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	ga := &testGithubRebaseActivities{}
	ba := &testBuildNotifyActivities{}
	env.RegisterActivity(ga)
	env.RegisterActivity(ba)

	repo := github.Repo{
		Owner: "owner",
		Name:  "test",
	}

	root := terraform.Root{
		Path:         "test/dir2",
		WhenModified: raw.DefaultAutoPlanWhenModified,
	}

	req := shouldRebaseRequest{
		Repo: repo,
		Root: root,
	}

	pullRequests := []github.PullRequest{
		{
			Number: 1,
		},
	}

	env.OnActivity(ga.GithubListOpenPRs, mock.Anything, activities.ListOpenPRsRequest{
		Repo: req.Repo,
	}).Return(activities.ListOpenPRsResponse{
		PullRequests: pullRequests,
	}, nil)

	env.OnActivity(ga.GithubListModifiedFiles, mock.Anything, activities.ListModifiedFilesRequest{
		Repo:        repo,
		PullRequest: pullRequests[0],
	}).Return(activities.ListModifiedFilesResponse{}, errors.New("err"))

	env.OnActivity(ba.BuildNotifyRebasePR, mock.Anything, activities.BuildNotifyRebasePRRequest{
		Repository:  repo,
		PullRequest: pullRequests[0],
	}).Return(activities.BuildNotifyRebasePRResponse{}, nil)

	env.ExecuteWorkflow(testShouldRebaseWorkflow, req)
	env.AssertExpectations(t)

	err := env.GetWorkflowResult(nil)
	assert.Nil(t, err)
}

func TestPullRebasePRs_FilePathMatchError_NeedRebase(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	ga := &testGithubRebaseActivities{}
	ba := &testBuildNotifyActivities{}
	env.RegisterActivity(ga)
	env.RegisterActivity(ba)

	repo := github.Repo{
		Owner: "owner",
		Name:  "test",
	}

	// invalid when modified config to simulate filepath match error
	root := terraform.Root{
		WhenModified: []string{"!"},
	}

	req := shouldRebaseRequest{
		Repo: repo,
		Root: root,
	}

	pullRequests := []github.PullRequest{
		{
			Number: 1,
		},
	}

	filesModifiedPr1 := []string{"test/dir2/rebase.tf"}

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

	env.OnActivity(ba.BuildNotifyRebasePR, mock.Anything, activities.BuildNotifyRebasePRRequest{
		Repository:  repo,
		PullRequest: pullRequests[0],
	}).Return(activities.BuildNotifyRebasePRResponse{}, nil)

	env.ExecuteWorkflow(testShouldRebaseWorkflow, req)
	env.AssertExpectations(t)

	err := env.GetWorkflowResult(nil)
	assert.Nil(t, err)
}
