package prrevision

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/runatlantis/atlantis/server/core/config/raw"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/prrevision/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func Test_ShouldSetMinimumRevisionForPR(t *testing.T) {
	cases := []struct {
		description   string
		root          terraform.Root
		modifiedFiles []string
		shouldReabse  bool
	}{
		{
			description: "default tracked files config, root dir modified",
			root: terraform.Root{
				Path:         "test/dir1",
				TrackedFiles: raw.DefaultAutoPlanWhenModified,
			},
			modifiedFiles: []string{"test/dir1/main.tf"},
			shouldReabse:  true,
		},
		{
			description: "default tracked files config, root dir not modified",
			root: terraform.Root{
				Path:         "test/dir1",
				TrackedFiles: raw.DefaultAutoPlanWhenModified,
			},
			modifiedFiles: []string{"test/dir2/main.tf"},
			shouldReabse:  false,
		},
		{
			description: "default tracked files config, .tfvars file modified",
			root: terraform.Root{
				Path:         "test/dir1",
				TrackedFiles: raw.DefaultAutoPlanWhenModified,
			},
			modifiedFiles: []string{"test/dir1/terraform.tfvars"},
			shouldReabse:  true,
		},
		{
			description: "non default tracked files config, non root dir modified",
			root: terraform.Root{
				Path:         "test/dir1",
				TrackedFiles: []string{"**/*.tf*", "../variables.tf"},
			},
			modifiedFiles: []string{"test/variables.tf"},
			shouldReabse:  true,
		},
		{
			description: "non default tracked files config, file excluded",
			root: terraform.Root{
				Path:         "test/dir1",
				TrackedFiles: []string{"**/*.tf*", "!exclude.tf"},
			},
			modifiedFiles: []string{"test/dir1/exclude.tf"},
			shouldReabse:  false,
		},
		{
			description: "non default tracked files config, file excluded",
			root: terraform.Root{
				Path:         "test/dir1",
				TrackedFiles: []string{"**/*.tf*", "!exclude.tf"},
			},
			modifiedFiles: []string{"test/dir1/exclude.tf"},
			shouldReabse:  false,
		},
		{
			description: "non default tracked files config, file excluded and included",
			root: terraform.Root{
				Path:         "test/dir1",
				TrackedFiles: []string{"**/*.tf*", "!exclude.tf"},
			},
			modifiedFiles: []string{"test/dir1/exclude.tf", "test/dir1/main.tf"},
			shouldReabse:  true,
		},
	}

	for _, c := range cases {
		res, err := isRootModified(c.root, c.modifiedFiles)
		assert.NoError(t, err)
		assert.Equal(t, c.shouldReabse, res)
	}
}

type testRevisionSetterActivities struct{}

func (t *testRevisionSetterActivities) SetPRRevision(ctx context.Context, request activities.SetPRRevisionRequest) error {
	return nil
}

type testGithubActivities struct{}

func (t *testGithubActivities) GithubListPRs(ctx context.Context, request activities.ListPRsRequest) (activities.ListPRsResponse, error) {
	return activities.ListPRsResponse{}, nil
}

func (t *testGithubActivities) GithubListModifiedFiles(ctx context.Context, request activities.ListModifiedFilesRequest) (activities.ListModifiedFilesResponse, error) {
	return activities.ListModifiedFilesResponse{}, nil
}

func testSetMiminumValidRevisionForRootWorkflow(ctx workflow.Context, r Request, slowProcessingCutOffDays int) error {
	options := workflow.ActivityOptions{
		ScheduleToCloseTimeout: 5 * time.Second,
		TaskQueue:              TaskQueue,
	}
	ctx = workflow.WithActivityOptions(ctx, options)

	runner := Runner{
		GithubActivities:         &testGithubActivities{},
		RevisionSetterActivities: &testRevisionSetterActivities{},
		Scope:                    metrics.NewNullableScope(),
		SlowProcessingCutOffDays: slowProcessingCutOffDays,
	}
	return runner.Run(ctx, r)
}

func TestMinRevisionSetter_NoOpenPR(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.OnGetVersion(version.MultiQueue, workflow.DefaultVersion, 1).Return(workflow.DefaultVersion)

	ga := &testGithubActivities{}
	env.RegisterActivity(ga)

	req := Request{
		Repo: github.Repo{
			Owner: "owner",
			Name:  "test",
		},
		Root: terraform.Root{
			Name: "test",
		},
	}

	env.OnActivity(ga.GithubListPRs, mock.Anything, activities.ListPRsRequest{
		Repo:  req.Repo,
		State: github.OpenPullRequest,
	}).Return(activities.ListPRsResponse{
		PullRequests: []github.PullRequest{},
	}, nil)

	env.ExecuteWorkflow(testSetMiminumValidRevisionForRootWorkflow, req, 10)
	env.AssertExpectations(t)

	err := env.GetWorkflowResult(nil)
	assert.Nil(t, err)
}

func TestMinRevisionSetter_OpenPR_SetMinRevision_Default(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.OnGetVersion(version.MultiQueue, workflow.DefaultVersion, 1).Return(workflow.DefaultVersion)

	ga := &testGithubActivities{}
	ra := &testRevisionSetterActivities{}
	env.RegisterActivity(ra)
	env.RegisterActivity(ga)

	req := Request{
		Repo: github.Repo{
			Owner: "owner",
			Name:  "test",
		},
		Root: terraform.Root{
			Path:         "test/dir2",
			TrackedFiles: raw.DefaultAutoPlanWhenModified,
		},
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

	env.OnActivity(ga.GithubListPRs, mock.Anything, activities.ListPRsRequest{
		Repo:  req.Repo,
		State: github.OpenPullRequest,
	}).Return(activities.ListPRsResponse{
		PullRequests: pullRequests,
	}, nil)

	env.OnActivity(ga.GithubListModifiedFiles, mock.Anything, activities.ListModifiedFilesRequest{
		Repo:        req.Repo,
		PullRequest: pullRequests[0],
	}).Return(activities.ListModifiedFilesResponse{
		FilePaths: filesModifiedPr1,
	}, nil)

	env.OnActivity(ga.GithubListModifiedFiles, mock.Anything, activities.ListModifiedFilesRequest{
		Repo:        req.Repo,
		PullRequest: pullRequests[1],
	}).Return(activities.ListModifiedFilesResponse{
		FilePaths: filesModifiedPr2,
	}, nil)

	env.OnActivity(ra.SetPRRevision, mock.Anything, activities.SetPRRevisionRequest{
		Repository:  req.Repo,
		PullRequest: pullRequests[0],
	}).Return(nil)

	env.ExecuteWorkflow(testSetMiminumValidRevisionForRootWorkflow, req, 10)
	env.AssertExpectations(t)

	err := env.GetWorkflowResult(nil)
	assert.Nil(t, err)
}

func TestMinRevisionSetter_OpenPR_SetMinRevision_v1(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	ga := &testGithubActivities{}
	ra := &testRevisionSetterActivities{}
	env.RegisterActivity(ra)
	env.RegisterActivity(ga)

	req := Request{
		Repo: github.Repo{
			Owner: "owner",
			Name:  "test",
		},
		Root: terraform.Root{
			Path:         "test/dir2",
			TrackedFiles: raw.DefaultAutoPlanWhenModified,
		},
	}

	now := time.Now()
	newPR := github.PullRequest{
		Number:    1,
		UpdatedAt: now.AddDate(0, 0, -5),
	}
	oldPR := github.PullRequest{
		Number:    2,
		UpdatedAt: now.AddDate(0, 0, -15),
	}
	pullRequests := []github.PullRequest{
		newPR, oldPR,
	}

	filesModifiedPr1 := []string{"test/dir2/rebase.tf"}
	filesModifiedPr2 := []string{"test/dir1/no-rebase.tf"}

	env.OnActivity(ga.GithubListPRs, mock.Anything, activities.ListPRsRequest{
		Repo:    req.Repo,
		State:   github.OpenPullRequest,
		SortKey: github.Updated,
		Order:   github.Descending,
	}).Return(activities.ListPRsResponse{
		PullRequests: pullRequests,
	}, nil)

	env.OnActivity(ga.GithubListModifiedFiles, mock.Anything, activities.ListModifiedFilesRequest{
		Repo:        req.Repo,
		PullRequest: newPR,
	}).Return(func(ctx context.Context, request activities.ListModifiedFilesRequest) (activities.ListModifiedFilesResponse, error) {
		assert.Equal(t, TaskQueue, activity.GetInfo(ctx).TaskQueue)
		return activities.ListModifiedFilesResponse{
			FilePaths: filesModifiedPr1,
		}, nil
	})

	env.OnActivity(ga.GithubListModifiedFiles, mock.Anything, activities.ListModifiedFilesRequest{
		Repo:        req.Repo,
		PullRequest: oldPR,
	}).Return(func(ctx context.Context, request activities.ListModifiedFilesRequest) (activities.ListModifiedFilesResponse, error) {
		assert.Equal(t, SlowTaskQueue, activity.GetInfo(ctx).TaskQueue)
		return activities.ListModifiedFilesResponse{
			FilePaths: filesModifiedPr2,
		}, nil
	})

	env.OnActivity(ra.SetPRRevision, mock.Anything, activities.SetPRRevisionRequest{
		Repository:  req.Repo,
		PullRequest: pullRequests[0],
	}).Return(nil)

	env.ExecuteWorkflow(testSetMiminumValidRevisionForRootWorkflow, req, 10)
	env.AssertExpectations(t)

	err := env.GetWorkflowResult(nil)
	assert.Nil(t, err)
}

func TestMinRevisionSetter_ListModifiedFilesErr(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.OnGetVersion(version.MultiQueue, workflow.DefaultVersion, 1).Return(workflow.DefaultVersion)

	ga := &testGithubActivities{}
	ra := &testRevisionSetterActivities{}
	env.RegisterActivity(ra)
	env.RegisterActivity(ga)

	req := Request{
		Repo: github.Repo{
			Owner: "owner",
			Name:  "test",
		},
		Root: terraform.Root{
			Path:         "test/dir2",
			TrackedFiles: raw.DefaultAutoPlanWhenModified,
		},
	}

	pullRequests := []github.PullRequest{
		{
			Number: 1,
		},
	}

	env.OnActivity(ga.GithubListPRs, mock.Anything, activities.ListPRsRequest{
		Repo:  req.Repo,
		State: github.OpenPullRequest,
	}).Return(activities.ListPRsResponse{
		PullRequests: pullRequests,
	}, nil)

	env.OnActivity(ga.GithubListModifiedFiles, mock.Anything, activities.ListModifiedFilesRequest{
		Repo:        req.Repo,
		PullRequest: pullRequests[0],
	}).Return(activities.ListModifiedFilesResponse{}, errors.New("error"))

	env.OnActivity(ra.SetPRRevision, mock.Anything, activities.SetPRRevisionRequest{
		Repository:  req.Repo,
		PullRequest: pullRequests[0],
	}).Return(nil)

	env.ExecuteWorkflow(testSetMiminumValidRevisionForRootWorkflow, req, 10)
	env.AssertExpectations(t)

	err := env.GetWorkflowResult(nil)
	assert.Nil(t, err)
}

func TestMinRevisionSetter_OpenPR_PatternMatchErr(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.OnGetVersion(version.MultiQueue, workflow.DefaultVersion, 1).Return(workflow.DefaultVersion)

	ga := &testGithubActivities{}
	ra := &testRevisionSetterActivities{}
	env.RegisterActivity(ra)
	env.RegisterActivity(ga)

	req := Request{
		Repo: github.Repo{
			Owner: "owner",
			Name:  "test",
		},
		Root: terraform.Root{
			TrackedFiles: []string{"!"},
		},
	}

	pullRequests := []github.PullRequest{
		{
			Number: 1,
		},
	}

	env.OnActivity(ga.GithubListPRs, mock.Anything, activities.ListPRsRequest{
		Repo:  req.Repo,
		State: github.OpenPullRequest,
	}).Return(activities.ListPRsResponse{
		PullRequests: pullRequests,
	}, nil)

	filesModifiedPr1 := []string{"test/dir2/rebase.tf"}
	env.OnActivity(ga.GithubListModifiedFiles, mock.Anything, activities.ListModifiedFilesRequest{
		Repo:        req.Repo,
		PullRequest: pullRequests[0],
	}).Return(activities.ListModifiedFilesResponse{
		FilePaths: filesModifiedPr1,
	}, nil)

	env.ExecuteWorkflow(testSetMiminumValidRevisionForRootWorkflow, req, 10)
	env.AssertExpectations(t)

	err := env.GetWorkflowResult(nil)
	assert.NoError(t, err)
}

func TestIsPrUpdatedWithinDays(t *testing.T) {
	now := time.Now()
	runner := Runner{}

	t.Run("updated before", func(t *testing.T) {
		prUpdatedLongAgo := github.PullRequest{
			UpdatedAt: now.AddDate(0, 0, -40),
		}

		assert.False(t, runner.isPrUpdatedWithinDays(now, prUpdatedLongAgo, 10))
	})

	t.Run("updated at", func(t *testing.T) {
		prUpdatedLongAgo := github.PullRequest{
			UpdatedAt: now.AddDate(0, 0, -10),
		}

		assert.False(t, runner.isPrUpdatedWithinDays(now, prUpdatedLongAgo, 10))
	})

	t.Run("updated after", func(t *testing.T) {
		prUpdatedLongAgo := github.PullRequest{
			UpdatedAt: now.AddDate(0, 0, -10),
		}

		assert.True(t, runner.isPrUpdatedWithinDays(now, prUpdatedLongAgo, 20))
	})
}
