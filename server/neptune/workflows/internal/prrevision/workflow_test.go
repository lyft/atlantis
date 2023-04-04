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

type testGithubActivityExecutor struct {
	t           *testing.T
	exepctedTQs map[int]string
	ga          *testGithubActivities
}

func (t *testGithubActivityExecutor) GithubListModifiedFiles(ctx workflow.Context, taskqueue string, request activities.ListModifiedFilesRequest) workflow.Future {
	assert.Equal(t.t, t.exepctedTQs[request.PullRequest.Number], taskqueue)
	return workflow.ExecuteActivity(ctx, t.ga.GithubListModifiedFiles, request)
}

func (t *testGithubActivityExecutor) GithubListPRs(ctx workflow.Context, request activities.ListPRsRequest) workflow.Future {
	return workflow.ExecuteActivity(ctx, t.ga.GithubListPRs, request)
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

type testRequest struct {
	UnderlyingReq            Request
	T                        *testing.T
	ExpectedTQs              map[int]string
	SlowProcessingCutOffDays int
}

func testSetMiminumValidRevisionForRootWorkflow(ctx workflow.Context, r testRequest) error {
	options := workflow.ActivityOptions{
		ScheduleToCloseTimeout: 5 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, options)

	runner := Runner{
		GithubActivityExecutor: &testGithubActivityExecutor{
			t:           r.T,
			ga:          &testGithubActivities{},
			exepctedTQs: r.ExpectedTQs,
		},
		RevisionSetterActivities: &testRevisionSetterActivities{},
		Scope:                    metrics.NewNullableScope(),
		SlowProcessingCutOffDays: r.SlowProcessingCutOffDays,
	}
	return runner.Run(ctx, r.UnderlyingReq)
}

func TestMinRevisionSetter_NoOpenPR(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	ga := &testGithubActivities{}
	env.RegisterActivity(ga)

	req := testRequest{
		T: t,
		UnderlyingReq: Request{
			Repo: github.Repo{
				Owner: "owner",
				Name:  "test",
			},
			Root: terraform.Root{
				Name: "test",
			},
		},
		ExpectedTQs: map[int]string{},
	}

	env.OnActivity(ga.GithubListPRs, mock.Anything, activities.ListPRsRequest{
		Repo:      req.UnderlyingReq.Repo,
		State:     github.OpenPullRequest,
		SortKey:   github.Updated,
		Direction: github.Descending,
	}).Return(activities.ListPRsResponse{
		PullRequests: []github.PullRequest{},
	}, nil)

	env.ExecuteWorkflow(testSetMiminumValidRevisionForRootWorkflow, req)
	env.AssertExpectations(t)

	err := env.GetWorkflowResult(nil)
	assert.Nil(t, err)
}

func TestMinRevisionSetter_OpenPR_SetMinRevision_Old(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	ga := &testGithubActivities{}
	ra := &testRevisionSetterActivities{}
	env.RegisterActivity(ra)
	env.RegisterActivity(ga)

	env.OnGetVersion(version.MultiQueue, workflow.DefaultVersion, 1).Return(workflow.DefaultVersion)

	pullRequests := []github.PullRequest{
		{
			Number:    1,
			UpdatedAt: time.Now().AddDate(0, 0, -15),
		},
		{
			Number:    2,
			UpdatedAt: time.Now().AddDate(0, 0, -5),
		},
	}

	req := testRequest{
		T: t,
		UnderlyingReq: Request{
			Repo: github.Repo{
				Owner: "owner",
				Name:  "test",
			},
			Root: terraform.Root{
				Path:         "test/dir2",
				TrackedFiles: raw.DefaultAutoPlanWhenModified,
			},
		},
		ExpectedTQs: map[int]string{
			pullRequests[0].Number: TaskQueue,
			pullRequests[1].Number: TaskQueue,
		},
	}

	filesModifiedPr1 := []string{"test/dir2/rebase.tf"}
	filesModifiedPr2 := []string{"test/dir1/no-rebase.tf"}

	env.OnActivity(ga.GithubListPRs, mock.Anything, activities.ListPRsRequest{
		Repo:      req.UnderlyingReq.Repo,
		State:     github.OpenPullRequest,
		SortKey:   github.Updated,
		Direction: github.Descending,
	}).Return(activities.ListPRsResponse{
		PullRequests: pullRequests,
	}, nil)

	env.OnActivity(ga.GithubListModifiedFiles, mock.Anything, activities.ListModifiedFilesRequest{
		Repo:        req.UnderlyingReq.Repo,
		PullRequest: pullRequests[0],
	}).Return(activities.ListModifiedFilesResponse{
		FilePaths: filesModifiedPr1,
	}, nil)

	env.OnActivity(ga.GithubListModifiedFiles, mock.Anything, activities.ListModifiedFilesRequest{
		Repo:        req.UnderlyingReq.Repo,
		PullRequest: pullRequests[1],
	}).Return(activities.ListModifiedFilesResponse{
		FilePaths: filesModifiedPr2,
	}, nil)

	env.OnActivity(ra.SetPRRevision, mock.Anything, activities.SetPRRevisionRequest{
		Repository:  req.UnderlyingReq.Repo,
		PullRequest: pullRequests[0],
	}).Return(nil)

	env.ExecuteWorkflow(testSetMiminumValidRevisionForRootWorkflow, req)
	env.AssertExpectations(t)

	err := env.GetWorkflowResult(nil)
	assert.Nil(t, err)
}

func TestMinRevisionSetter_OpenPR_SetMinRevision_MultiTQ(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	ga := &testGithubActivities{}
	ra := &testRevisionSetterActivities{}
	env.RegisterActivity(ra)
	env.RegisterActivity(ga)

	pullRequests := []github.PullRequest{
		{
			Number:    1,
			UpdatedAt: time.Now().AddDate(0, 0, -5),
		},
		{
			Number:    2,
			UpdatedAt: time.Now().AddDate(0, 0, -15),
		},
	}

	req := testRequest{
		T: t,
		UnderlyingReq: Request{
			Repo: github.Repo{
				Owner: "owner",
				Name:  "test",
			},
			Root: terraform.Root{
				Path:         "test/dir2",
				TrackedFiles: raw.DefaultAutoPlanWhenModified,
			},
		},
		SlowProcessingCutOffDays: 10,
		ExpectedTQs: map[int]string{
			pullRequests[0].Number: TaskQueue,

			// switch to slow task queue when older than cut off days
			pullRequests[1].Number: SlowTaskQueue,
		},
	}

	filesModifiedPr1 := []string{"test/dir2/rebase.tf"}
	filesModifiedPr2 := []string{"test/dir1/no-rebase.tf"}

	env.OnActivity(ga.GithubListPRs, mock.Anything, activities.ListPRsRequest{
		Repo:      req.UnderlyingReq.Repo,
		State:     github.OpenPullRequest,
		SortKey:   github.Updated,
		Direction: github.Descending,
	}).Return(activities.ListPRsResponse{
		PullRequests: pullRequests,
	}, nil)

	env.OnActivity(ga.GithubListModifiedFiles, mock.Anything, activities.ListModifiedFilesRequest{
		Repo:        req.UnderlyingReq.Repo,
		PullRequest: pullRequests[0],
	}).Return(activities.ListModifiedFilesResponse{
		FilePaths: filesModifiedPr1,
	}, nil)

	env.OnActivity(ga.GithubListModifiedFiles, mock.Anything, activities.ListModifiedFilesRequest{
		Repo:        req.UnderlyingReq.Repo,
		PullRequest: pullRequests[1],
	}).Return(activities.ListModifiedFilesResponse{
		FilePaths: filesModifiedPr2,
	}, nil)

	env.OnActivity(ra.SetPRRevision, mock.Anything, activities.SetPRRevisionRequest{
		Repository:  req.UnderlyingReq.Repo,
		PullRequest: pullRequests[0],
	}).Return(nil)

	env.ExecuteWorkflow(testSetMiminumValidRevisionForRootWorkflow, req)
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

	pullRequests := []github.PullRequest{
		{
			Number: 1,
		},
	}

	req := testRequest{
		T: t,
		UnderlyingReq: Request{
			Repo: github.Repo{
				Owner: "owner",
				Name:  "test",
			},
			Root: terraform.Root{
				Path:         "test/dir2",
				TrackedFiles: raw.DefaultAutoPlanWhenModified,
			},
		},
		ExpectedTQs: map[int]string{
			pullRequests[0].Number: TaskQueue,
		},
	}

	env.OnActivity(ga.GithubListPRs, mock.Anything, activities.ListPRsRequest{
		Repo:      req.UnderlyingReq.Repo,
		State:     github.OpenPullRequest,
		SortKey:   github.Updated,
		Direction: github.Descending,
	}).Return(activities.ListPRsResponse{
		PullRequests: pullRequests,
	}, nil)

	env.OnActivity(ga.GithubListModifiedFiles, mock.Anything, activities.ListModifiedFilesRequest{
		Repo:        req.UnderlyingReq.Repo,
		PullRequest: pullRequests[0],
	}).Return(activities.ListModifiedFilesResponse{}, errors.New("error"))

	env.OnActivity(ra.SetPRRevision, mock.Anything, activities.SetPRRevisionRequest{
		Repository:  req.UnderlyingReq.Repo,
		PullRequest: pullRequests[0],
	}).Return(nil)

	env.ExecuteWorkflow(testSetMiminumValidRevisionForRootWorkflow, req)
	env.AssertExpectations(t)

	err := env.GetWorkflowResult(nil)
	assert.Nil(t, err)
}

func TestMinRevisionSetter_OpenPR_PatternMatchErr(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	ga := &testGithubActivities{}
	ra := &testRevisionSetterActivities{}
	env.RegisterActivity(ra)
	env.RegisterActivity(ga)

	pullRequests := []github.PullRequest{
		{
			Number: 1,
		},
	}

	req := testRequest{
		T: t,
		UnderlyingReq: Request{
			Repo: github.Repo{
				Owner: "owner",
				Name:  "test",
			},
			Root: terraform.Root{
				TrackedFiles: []string{"!"},
			},
		},
		ExpectedTQs: map[int]string{
			pullRequests[0].Number: TaskQueue,
		},
	}

	env.OnActivity(ga.GithubListPRs, mock.Anything, activities.ListPRsRequest{
		Repo:      req.UnderlyingReq.Repo,
		State:     github.OpenPullRequest,
		SortKey:   github.Updated,
		Direction: github.Descending,
	}).Return(activities.ListPRsResponse{
		PullRequests: pullRequests,
	}, nil)

	filesModifiedPr1 := []string{"test/dir2/rebase.tf"}
	env.OnActivity(ga.GithubListModifiedFiles, mock.Anything, activities.ListModifiedFilesRequest{
		Repo:        req.UnderlyingReq.Repo,
		PullRequest: pullRequests[0],
	}).Return(activities.ListModifiedFilesResponse{
		FilePaths: filesModifiedPr1,
	}, nil)

	env.ExecuteWorkflow(testSetMiminumValidRevisionForRootWorkflow, req)
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
