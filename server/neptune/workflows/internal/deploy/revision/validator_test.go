package revision_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/root"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type testValidatorActivity struct{}

func (t *testValidatorActivity) CompareCommit(ctx context.Context, request activities.CompareCommitRequest) (activities.CompareCommitResponse, error) {
	return activities.CompareCommitResponse{}, nil
}

func (t *testValidatorActivity) UpdateCheckRun(ctx context.Context, request activities.UpdateCheckRunRequest) (activities.UpdateCheckRunResponse, error) {
	return activities.UpdateCheckRunResponse{}, nil
}

type testValidateWorklflowReq struct {
	Repo                   github.Repo
	DeployReqRevision      terraform.DeploymentInfo
	LatestDeployedRevision root.DeploymentInfo
}

func testValidatorWorkflow(ctx workflow.Context, r testValidateWorklflowReq) (bool, error) {
	var ga *testValidatorActivity

	validator := revision.Validator{
		Activity: ga,
	}

	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: 5 * time.Second,
	})

	return validator.IsValid(ctx, r.Repo, r.DeployReqRevision, &r.LatestDeployedRevision)
}

func TestValidator_IsRevisionValid(t *testing.T) {

	repo := github.Repo{
		Owner: "test",
		Name:  "repo",
	}

	t.Run("deploy request revison is ahead of latest deployed revision", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()

		ta := &testValidatorActivity{}
		deployReqRevision := terraform.DeploymentInfo{
			Root: root.Root{
				Name: "test-root",
			},
			Revision:   "1234",
			CheckRunID: 6789,
		}

		latestDeployedRevision := root.DeploymentInfo{
			Revision: "4567",
		}

		env.OnActivity(ta.CompareCommit, mock.Anything, activities.CompareCommitRequest{
			Repo:                   repo,
			DeployRequestRevision:  deployReqRevision.Revision,
			LatestDeployedRevision: latestDeployedRevision.Revision,
		}).Return(activities.CompareCommitResponse{
			CommitComparison: activities.DirectionAhead,
		}, nil)

		env.ExecuteWorkflow(testValidatorWorkflow, testValidateWorklflowReq{
			Repo:                   repo,
			DeployReqRevision:      deployReqRevision,
			LatestDeployedRevision: latestDeployedRevision,
		})

		var resp bool
		err := env.GetWorkflowResult(&resp)
		assert.NoError(t, err)
		assert.Equal(t, true, resp)
	})

	t.Run("deploy request revison is same as the latest deployed revision", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()

		ta := &testValidatorActivity{}
		deployReqRevision := terraform.DeploymentInfo{
			Root: root.Root{
				Name: "test-root",
			},
			Revision:   "1234",
			CheckRunID: 6789,
		}

		latestDeployedRevision := root.DeploymentInfo{
			Revision: "1234",
		}

		env.OnActivity(ta.UpdateCheckRun, mock.Anything, activities.UpdateCheckRunRequest{
			Title:   terraform.BuildCheckRunTitle(deployReqRevision.Root.Name),
			State:   github.CheckRunSuccess,
			Repo:    repo,
			ID:      deployReqRevision.CheckRunID,
			Summary: revision.IdenticalRevisonSummary,
		}).Return(activities.UpdateCheckRunResponse{
			ID: deployReqRevision.CheckRunID,
		}, nil)

		env.ExecuteWorkflow(testValidatorWorkflow, testValidateWorklflowReq{
			Repo:                   repo,
			DeployReqRevision:      deployReqRevision,
			LatestDeployedRevision: latestDeployedRevision,
		})

		var resp bool
		err := env.GetWorkflowResult(&resp)
		assert.NoError(t, err)
		assert.Equal(t, false, resp)
	})

	t.Run("deploy request revison is behind the latest deployed revision", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()

		ta := &testValidatorActivity{}
		deployReqRevision := terraform.DeploymentInfo{
			Root: root.Root{
				Name: "test-root",
			},
			Revision:   "1234",
			CheckRunID: 6789,
		}

		latestDeployedRevision := root.DeploymentInfo{
			Revision: "3455",
		}

		env.OnActivity(ta.CompareCommit, mock.Anything, activities.CompareCommitRequest{
			Repo:                   repo,
			DeployRequestRevision:  deployReqRevision.Revision,
			LatestDeployedRevision: latestDeployedRevision.Revision,
		}).Return(activities.CompareCommitResponse{
			CommitComparison: activities.DirectionBehind,
		}, nil)

		env.OnActivity(ta.UpdateCheckRun, mock.Anything, activities.UpdateCheckRunRequest{
			Title:   terraform.BuildCheckRunTitle(deployReqRevision.Root.Name),
			State:   github.CheckRunSuccess,
			Repo:    repo,
			ID:      deployReqRevision.CheckRunID,
			Summary: revision.DirectionBehindSummary,
		}).Return(activities.UpdateCheckRunResponse{
			ID: deployReqRevision.CheckRunID,
		}, nil)

		env.ExecuteWorkflow(testValidatorWorkflow, testValidateWorklflowReq{
			Repo:                   repo,
			DeployReqRevision:      deployReqRevision,
			LatestDeployedRevision: latestDeployedRevision,
		})

		var resp bool
		err := env.GetWorkflowResult(&resp)
		assert.NoError(t, err)
		assert.Equal(t, false, resp)
	})

	t.Run("deploy request revison is idenmtical to the latest deployed revision", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()

		ta := &testValidatorActivity{}
		deployReqRevision := terraform.DeploymentInfo{
			Root: root.Root{
				Name: "test-root",
			},
			Revision:   "1234",
			CheckRunID: 6789,
		}

		latestDeployedRevision := root.DeploymentInfo{
			Revision: "3455",
		}

		env.OnActivity(ta.CompareCommit, mock.Anything, activities.CompareCommitRequest{
			Repo:                   repo,
			DeployRequestRevision:  deployReqRevision.Revision,
			LatestDeployedRevision: latestDeployedRevision.Revision,
		}).Return(activities.CompareCommitResponse{
			CommitComparison: activities.DirectionIdentical,
		}, nil)

		env.OnActivity(ta.UpdateCheckRun, mock.Anything, activities.UpdateCheckRunRequest{
			Title:   terraform.BuildCheckRunTitle(deployReqRevision.Root.Name),
			State:   github.CheckRunSuccess,
			Repo:    repo,
			ID:      deployReqRevision.CheckRunID,
			Summary: revision.IdenticalRevisonSummary,
		}).Return(activities.UpdateCheckRunResponse{
			ID: deployReqRevision.CheckRunID,
		}, nil)

		env.ExecuteWorkflow(testValidatorWorkflow, testValidateWorklflowReq{
			Repo:                   repo,
			DeployReqRevision:      deployReqRevision,
			LatestDeployedRevision: latestDeployedRevision,
		})

		var resp bool
		err := env.GetWorkflowResult(&resp)
		assert.NoError(t, err)
		assert.Equal(t, false, resp)
	})

	t.Run("compare commit activity errors out", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()

		ta := &testValidatorActivity{}
		deployReqRevision := terraform.DeploymentInfo{
			Revision: "1234",
		}

		latestDeployedRevision := root.DeploymentInfo{
			Revision: "4564",
		}

		activityErr := errors.New("error")
		env.OnActivity(ta.CompareCommit, mock.Anything, activities.CompareCommitRequest{
			Repo:                   repo,
			DeployRequestRevision:  deployReqRevision.Revision,
			LatestDeployedRevision: latestDeployedRevision.Revision,
		}).Return(activities.CompareCommitResponse{}, activityErr)

		env.ExecuteWorkflow(testValidatorWorkflow, testValidateWorklflowReq{
			Repo:                   repo,
			DeployReqRevision:      deployReqRevision,
			LatestDeployedRevision: latestDeployedRevision,
		})

		var resp bool
		err := env.GetWorkflowResult(&resp)
		assert.ErrorContains(t, err, activityErr.Error())
		assert.Equal(t, false, resp)
	})

}
