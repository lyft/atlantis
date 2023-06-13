package queue_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/notifier"
	"github.com/runatlantis/atlantis/server/neptune/workflows/plugins"

	"github.com/google/uuid"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/deployment"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	model "github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision/queue"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/version"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type ErrorType string

const (
	PlanRejectionError   ErrorType = "PlanRejectionError"
	TerraformClientError ErrorType = "TerraformClientError"
)

type testExecutor struct {
	called bool
}

func (e *testExecutor) Execute(ctx workflow.Context, info plugins.TerraformDeploymentInfo) error {
	e.called = true
	return nil
}

type testTerraformWorkflowRunner struct {
	expectedDeployment terraform.DeploymentInfo
	expectedErrorType  ErrorType
}

func (r testTerraformWorkflowRunner) Run(ctx workflow.Context, deploymentInfo terraform.DeploymentInfo, PlanApproval model.PlanApproval, scope metrics.Scope) error {
	if r.expectedErrorType == PlanRejectionError {
		return terraform.NewPlanRejectionError("plan rejected")
	} else if r.expectedErrorType == TerraformClientError {
		return activities.NewTerraformClientError(errors.New("error"))
	}
	return nil
}

type testDeployActivity struct{}

func (t *testDeployActivity) FetchLatestDeployment(ctx context.Context, deployerRequest activities.FetchLatestDeploymentRequest) (activities.FetchLatestDeploymentResponse, error) {
	return activities.FetchLatestDeploymentResponse{}, nil
}

func (t *testDeployActivity) StoreLatestDeployment(ctx context.Context, deployerRequest activities.StoreLatestDeploymentRequest) error {
	return nil
}

func (t *testDeployActivity) GithubCompareCommit(ctx context.Context, deployerRequest activities.CompareCommitRequest) (activities.CompareCommitResponse, error) {
	return activities.CompareCommitResponse{}, nil
}

func (t *testDeployActivity) GithubUpdateCheckRun(ctx context.Context, deployerRequest activities.UpdateCheckRunRequest) (activities.UpdateCheckRunResponse, error) {
	return activities.UpdateCheckRunResponse{}, nil
}

type deployerRequest struct {
	Info              terraform.DeploymentInfo
	LatestDeploy      *deployment.Info
	ErrType           ErrorType
	ExpectedGHRequest notifier.GithubCheckRunRequest
	ExpectedT         *testing.T
}

type deployResponse struct {
	*deployment.Info
	ExecutorCalled bool
}

func testDeployerWorkflow(ctx workflow.Context, r deployerRequest) (*deployResponse, error) {
	options := workflow.ActivityOptions{
		ScheduleToCloseTimeout: 5 * time.Second,
	}

	ctx = workflow.WithActivityOptions(ctx, options)
	var a *testDeployActivity

	e := &testExecutor{}

	deployer := &queue.Deployer{
		Activities: a,
		TerraformWorkflowRunner: &testTerraformWorkflowRunner{
			expectedDeployment: r.Info,
			expectedErrorType:  r.ErrType,
		},
		GithubCheckRunCache: &testCheckRunClient{
			expectedRequest:      r.ExpectedGHRequest,
			expectedT:            r.ExpectedT,
			expectedDeploymentID: r.Info.ID.String(),
		},
		Executors: []plugins.PostDeployExecutor{e},
	}

	info, err := deployer.Deploy(ctx, r.Info, r.LatestDeploy, metrics.NewNullableScope())
	if err != nil {
		return nil, err
	}

	return &deployResponse{Info: info, ExecutorCalled: e.called}, nil
}

func TestDeployer_FirstDeploy(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.OnGetVersion(version.SetPRRevision, workflow.DefaultVersion, 2).Return(workflow.DefaultVersion)

	da := &testDeployActivity{}
	env.RegisterActivity(da)

	repo := github.Repo{
		Owner: "owner",
		Name:  "test",
	}

	root := model.Root{
		Name: "root_1",
	}

	deploymentInfo := terraform.DeploymentInfo{
		ID: uuid.UUID{},
		Commit: github.Commit{
			Revision: "3455",
			Branch:   "default-branch",
		},
		CheckRunID: 1234,
		Root:       root,
		Repo:       repo,
	}

	latestDeployedRevision := &deployment.Info{
		ID:       deploymentInfo.ID.String(),
		Version:  1.0,
		Revision: "3455",
		Branch:   "default-branch",
		Root: deployment.Root{
			Name: deploymentInfo.Root.Name,
		},
		Repo: deployment.Repo{
			Owner: deploymentInfo.Repo.Owner,
			Name:  deploymentInfo.Repo.Name,
		},
	}

	storeDeploymentRequest := activities.StoreLatestDeploymentRequest{
		DeploymentInfo: &deployment.Info{
			Version:  deployment.InfoSchemaVersion,
			ID:       deploymentInfo.ID.String(),
			Revision: deploymentInfo.Commit.Revision,
			Branch:   deploymentInfo.Commit.Branch,
			Root: deployment.Root{
				Name: deploymentInfo.Root.Name,
			},
			Repo: deployment.Repo{
				Owner: deploymentInfo.Repo.Owner,
				Name:  deploymentInfo.Repo.Name,
			},
		},
	}

	env.OnActivity(da.StoreLatestDeployment, mock.Anything, storeDeploymentRequest).Return(nil)

	env.ExecuteWorkflow(testDeployerWorkflow, deployerRequest{
		Info: deploymentInfo,
	})

	env.AssertExpectations(t)

	var resp *deployResponse
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)

	assert.Equal(t, latestDeployedRevision, resp.Info)
}

func TestDeployer_CompareCommit_DeployAhead(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.OnGetVersion(version.SetPRRevision, workflow.DefaultVersion, 2).Return(workflow.DefaultVersion)

	da := &testDeployActivity{}
	env.RegisterActivity(da)

	repo := github.Repo{
		Owner: "owner",
		Name:  "test",
	}

	root := model.Root{
		Name: "root_1",
	}

	deploymentInfo := terraform.DeploymentInfo{
		ID: uuid.UUID{},
		Commit: github.Commit{
			Revision: "3455",
			Branch:   "default-branch",
		},
		CheckRunID: 1234,
		Root:       root,
		Repo:       repo,
	}

	latestDeployedRevision := &deployment.Info{
		ID:       deploymentInfo.ID.String(),
		Version:  1.0,
		Revision: "3255",
		Branch:   "default-branch",
		Root: deployment.Root{
			Name: deploymentInfo.Root.Name,
		},
		Repo: deployment.Repo{
			Owner: deploymentInfo.Repo.Owner,
			Name:  deploymentInfo.Repo.Name,
		},
	}

	storeDeploymentRequest := activities.StoreLatestDeploymentRequest{
		DeploymentInfo: &deployment.Info{
			Version:  deployment.InfoSchemaVersion,
			ID:       deploymentInfo.ID.String(),
			Revision: deploymentInfo.Commit.Revision,
			Branch:   deploymentInfo.Commit.Branch,
			Root: deployment.Root{
				Name: deploymentInfo.Root.Name,
			},
			Repo: deployment.Repo{
				Owner: deploymentInfo.Repo.Owner,
				Name:  deploymentInfo.Repo.Name,
			},
		},
	}

	compareCommitRequest := activities.CompareCommitRequest{
		Repo:                   repo,
		DeployRequestRevision:  deploymentInfo.Commit.Revision,
		LatestDeployedRevision: latestDeployedRevision.Revision,
	}

	compareCommitResponse := activities.CompareCommitResponse{
		CommitComparison: activities.DirectionAhead,
	}

	env.OnActivity(da.GithubCompareCommit, mock.Anything, compareCommitRequest).Return(compareCommitResponse, nil)
	env.OnActivity(da.StoreLatestDeployment, mock.Anything, storeDeploymentRequest).Return(nil)

	env.ExecuteWorkflow(testDeployerWorkflow, deployerRequest{
		Info:         deploymentInfo,
		LatestDeploy: latestDeployedRevision,
	})

	env.AssertExpectations(t)

	var resp *deployResponse
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)

	assert.Equal(t, &deployment.Info{
		ID:       deploymentInfo.ID.String(),
		Version:  1.0,
		Revision: "3455",
		Branch:   "default-branch",
		Root: deployment.Root{
			Name: deploymentInfo.Root.Name,
		},
		Repo: deployment.Repo{
			Owner: deploymentInfo.Repo.Owner,
			Name:  deploymentInfo.Repo.Name,
		},
	}, resp.Info)
}

func TestDeployer_CompareCommit_Identical(t *testing.T) {
	repo := github.Repo{
		Owner: "owner",
		Name:  "test",
	}
	root := model.Root{
		Name: "root_1",
		TriggerInfo: model.TriggerInfo{
			Type:  model.ManualTrigger,
			Rerun: true,
		},
	}
	deploymentInfo := terraform.DeploymentInfo{
		ID: uuid.UUID{},
		Commit: github.Commit{
			Revision: "3455",
			Branch:   "default-branch",
		},
		CheckRunID: 1234,
		Root:       root,
		Repo:       repo,
	}

	latestDeployedRevision := &deployment.Info{
		ID:       deploymentInfo.ID.String(),
		Version:  1.0,
		Revision: "3255",
		Branch:   "default-branch",
		Root: deployment.Root{
			Name: deploymentInfo.Root.Name,
		},
		Repo: deployment.Repo{
			Owner: deploymentInfo.Repo.Owner,
			Name:  deploymentInfo.Repo.Name,
		},
	}
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.OnGetVersion(version.SetPRRevision, workflow.DefaultVersion, 2).Return(workflow.DefaultVersion)

	da := &testDeployActivity{}
	env.RegisterActivity(da)
	compareCommitRequest := activities.CompareCommitRequest{
		Repo:                   repo,
		DeployRequestRevision:  deploymentInfo.Commit.Revision,
		LatestDeployedRevision: latestDeployedRevision.Revision,
	}

	compareCommitResponse := activities.CompareCommitResponse{
		CommitComparison: activities.DirectionIdentical,
	}

	env.OnActivity(da.GithubCompareCommit, mock.Anything, compareCommitRequest).Return(compareCommitResponse, nil)
	env.ExecuteWorkflow(testDeployerWorkflow, deployerRequest{
		Info:         deploymentInfo,
		LatestDeploy: latestDeployedRevision,
	})
	env.AssertExpectations(t)
	var resp *deployResponse
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)
	assert.Equal(t, &deployment.Info{
		ID:       deploymentInfo.ID.String(),
		Version:  1.0,
		Revision: "3455",
		Branch:   "default-branch",
		Root: deployment.Root{
			Name:        deploymentInfo.Root.Name,
			Trigger:     "manual",
			ManualRerun: true,
		},
		Repo: deployment.Repo{
			Owner: deploymentInfo.Repo.Owner,
			Name:  deploymentInfo.Repo.Name,
		},
	}, resp.Info)
}

func TestDeployer_CompareCommit_SkipDeploy(t *testing.T) {
	repo := github.Repo{
		Owner: "owner",
		Name:  "test",
	}
	root := model.Root{
		Name: "root_1",
		TriggerInfo: model.TriggerInfo{
			Type:  model.ManualTrigger,
			Rerun: true,
		},
	}
	deploymentInfo := terraform.DeploymentInfo{
		ID: uuid.UUID{},
		Commit: github.Commit{
			Revision: "3455",
			Branch:   "default-branch",
		},
		CheckRunID: 1234,
		Root:       root,
		Repo:       repo,
	}

	latestDeployedRevision := &deployment.Info{
		ID:       deploymentInfo.ID.String(),
		Version:  1.0,
		Revision: "3255",
		Branch:   "default-branch",
		Root: deployment.Root{
			Name: deploymentInfo.Root.Name,
		},
		Repo: deployment.Repo{
			Owner: deploymentInfo.Repo.Owner,
			Name:  deploymentInfo.Repo.Name,
		},
	}

	t.Run("behind deploy", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()

		da := &testDeployActivity{}
		env.RegisterActivity(da)
		compareCommitRequest := activities.CompareCommitRequest{
			Repo:                   repo,
			DeployRequestRevision:  deploymentInfo.Commit.Revision,
			LatestDeployedRevision: latestDeployedRevision.Revision,
		}

		compareCommitResponse := activities.CompareCommitResponse{
			CommitComparison: activities.DirectionBehind,
		}

		env.OnActivity(da.GithubCompareCommit, mock.Anything, compareCommitRequest).Return(compareCommitResponse, nil)

		env.ExecuteWorkflow(testDeployerWorkflow, deployerRequest{
			Info:         deploymentInfo,
			LatestDeploy: latestDeployedRevision,
			ExpectedGHRequest: notifier.GithubCheckRunRequest{
				Title:   notifier.BuildDeployCheckRunTitle(deploymentInfo.Root.Name),
				State:   github.CheckRunFailure,
				Repo:    repo,
				Summary: queue.DirectionBehindSummary,
				Sha:     deploymentInfo.Commit.Revision,
			},
			ExpectedT: t,
		})
		env.AssertExpectations(t)
		err := env.GetWorkflowError()
		assert.Error(t, err)
	})

	cases := []activities.DiffDirection{activities.DirectionAhead, activities.DirectionDiverged}
	for _, diffDirection := range cases {
		t.Run(fmt.Sprintf("rerun deploy %s", diffDirection), func(t *testing.T) {
			ts := testsuite.WorkflowTestSuite{}
			env := ts.NewTestWorkflowEnvironment()

			da := &testDeployActivity{}
			env.RegisterActivity(da)
			compareCommitRequest := activities.CompareCommitRequest{
				Repo:                   repo,
				DeployRequestRevision:  deploymentInfo.Commit.Revision,
				LatestDeployedRevision: latestDeployedRevision.Revision,
			}

			compareCommitResponse := activities.CompareCommitResponse{
				CommitComparison: diffDirection,
			}

			env.OnActivity(da.GithubCompareCommit, mock.Anything, compareCommitRequest).Return(compareCommitResponse, nil)

			env.ExecuteWorkflow(testDeployerWorkflow, deployerRequest{
				Info:         deploymentInfo,
				LatestDeploy: latestDeployedRevision,
				ExpectedGHRequest: notifier.GithubCheckRunRequest{
					Title:   notifier.BuildDeployCheckRunTitle(deploymentInfo.Root.Name),
					State:   github.CheckRunFailure,
					Repo:    repo,
					Summary: queue.RerunNotIdenticalSummary,
					Sha:     deploymentInfo.Commit.Revision,
				},
				ExpectedT: t,
			})
			env.AssertExpectations(t)
			err := env.GetWorkflowError()
			assert.Error(t, err)
		})
	}
}

func TestDeployer_CompareCommit_DeployDiverged(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.OnGetVersion(version.SetPRRevision, workflow.DefaultVersion, 2).Return(workflow.DefaultVersion)

	da := &testDeployActivity{}
	env.RegisterActivity(da)

	repo := github.Repo{
		Owner: "owner",
		Name:  "test",
	}

	root := model.Root{
		Name: "root_1",
	}

	deploymentInfo := terraform.DeploymentInfo{
		ID: uuid.UUID{},
		Commit: github.Commit{
			Revision: "3455",
			Branch:   "default-branch",
		},
		CheckRunID: 1234,
		Root:       root,
		Repo:       repo,
	}

	latestDeployedRevision := &deployment.Info{
		ID:       deploymentInfo.ID.String(),
		Version:  1.0,
		Revision: "3255",
		Branch:   "default-branch",
		Root: deployment.Root{
			Name: deploymentInfo.Root.Name,
		},
		Repo: deployment.Repo{
			Owner: deploymentInfo.Repo.Owner,
			Name:  deploymentInfo.Repo.Name,
		},
	}

	storeDeploymentRequest := activities.StoreLatestDeploymentRequest{
		DeploymentInfo: &deployment.Info{
			Version:  deployment.InfoSchemaVersion,
			ID:       deploymentInfo.ID.String(),
			Revision: deploymentInfo.Commit.Revision,
			Branch:   deploymentInfo.Commit.Branch,
			Root: deployment.Root{
				Name: deploymentInfo.Root.Name,
			},
			Repo: deployment.Repo{
				Owner: deploymentInfo.Repo.Owner,
				Name:  deploymentInfo.Repo.Name,
			},
		},
	}

	compareCommitRequest := activities.CompareCommitRequest{
		Repo:                   repo,
		DeployRequestRevision:  deploymentInfo.Commit.Revision,
		LatestDeployedRevision: latestDeployedRevision.Revision,
	}

	compareCommitResponse := activities.CompareCommitResponse{
		CommitComparison: activities.DirectionDiverged,
	}

	env.OnActivity(da.GithubCompareCommit, mock.Anything, compareCommitRequest).Return(compareCommitResponse, nil)
	env.OnActivity(da.StoreLatestDeployment, mock.Anything, storeDeploymentRequest).Return(nil)

	env.ExecuteWorkflow(testDeployerWorkflow, deployerRequest{
		Info:         deploymentInfo,
		LatestDeploy: latestDeployedRevision,
	})

	env.AssertExpectations(t)

	var resp *deployResponse
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)

	assert.Equal(t, &deployment.Info{
		ID:       deploymentInfo.ID.String(),
		Version:  1.0,
		Revision: "3455",
		Branch:   "default-branch",
		Root: deployment.Root{
			Name: deploymentInfo.Root.Name,
		},
		Repo: deployment.Repo{
			Owner: deploymentInfo.Repo.Owner,
			Name:  deploymentInfo.Repo.Name,
		},
	}, resp.Info)
}

func TestDeployer_WorkflowFailure_PlanRejection_SkipUpdateLatestDeployment(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	da := &testDeployActivity{}
	env.RegisterActivity(da)

	repo := github.Repo{
		Owner: "owner",
		Name:  "test",
	}

	root := model.Root{
		Name: "root_1",
	}

	deploymentInfo := terraform.DeploymentInfo{
		ID: uuid.UUID{},
		Commit: github.Commit{
			Revision: "3455",
			Branch:   "default-branch",
		},
		CheckRunID: 1234,
		Root:       root,
		Repo:       repo,
	}

	latestDeployedRevision := &deployment.Info{
		ID:       deploymentInfo.ID.String(),
		Version:  1.0,
		Revision: "3255",
		Branch:   "default-branch",
		Root: deployment.Root{
			Name: deploymentInfo.Root.Name,
		},
		Repo: deployment.Repo{
			Owner: deploymentInfo.Repo.Owner,
			Name:  deploymentInfo.Repo.Name,
		},
	}

	compareCommitRequest := activities.CompareCommitRequest{
		Repo:                   repo,
		DeployRequestRevision:  deploymentInfo.Commit.Revision,
		LatestDeployedRevision: latestDeployedRevision.Revision,
	}

	compareCommitResponse := activities.CompareCommitResponse{
		CommitComparison: activities.DirectionAhead,
	}

	env.OnActivity(da.GithubCompareCommit, mock.Anything, compareCommitRequest).Return(compareCommitResponse, nil)

	env.ExecuteWorkflow(testDeployerWorkflow, deployerRequest{
		Info:         deploymentInfo,
		LatestDeploy: latestDeployedRevision,
		ErrType:      PlanRejectionError,
	})

	env.AssertExpectations(t)

	err := env.GetWorkflowError()

	wfErr, ok := err.(*temporal.WorkflowExecutionError)
	assert.True(t, ok)

	appErr, ok := wfErr.Unwrap().(*temporal.ApplicationError)
	assert.True(t, ok)

	receivedErrType := appErr.Type()

	assert.Equal(t, "PlanRejectionError", receivedErrType)
}

func TestDeployer_TerraformClientError_UpdateLatestDeployment(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.OnGetVersion(version.SetPRRevision, workflow.DefaultVersion, 2).Return(workflow.DefaultVersion)

	da := &testDeployActivity{}
	env.RegisterActivity(da)

	repo := github.Repo{
		Owner: "owner",
		Name:  "test",
	}

	root := model.Root{
		Name: "root_1",
	}

	deploymentInfo := terraform.DeploymentInfo{
		ID: uuid.UUID{},
		Commit: github.Commit{
			Revision: "3455",
			Branch:   "default-branch",
		},
		CheckRunID: 1234,
		Root:       root,
		Repo:       repo,
	}

	latestDeployedRevision := &deployment.Info{
		ID:       deploymentInfo.ID.String(),
		Version:  1.0,
		Revision: "3255",
		Branch:   "default-branch",
		Root: deployment.Root{
			Name: deploymentInfo.Root.Name,
		},
		Repo: deployment.Repo{
			Owner: deploymentInfo.Repo.Owner,
			Name:  deploymentInfo.Repo.Name,
		},
	}

	compareCommitRequest := activities.CompareCommitRequest{
		Repo:                   repo,
		DeployRequestRevision:  deploymentInfo.Commit.Revision,
		LatestDeployedRevision: latestDeployedRevision.Revision,
	}

	compareCommitResponse := activities.CompareCommitResponse{
		CommitComparison: activities.DirectionAhead,
	}

	storeDeploymentRequest := activities.StoreLatestDeploymentRequest{
		DeploymentInfo: &deployment.Info{
			Version:  deployment.InfoSchemaVersion,
			ID:       deploymentInfo.ID.String(),
			Revision: deploymentInfo.Commit.Revision,
			Branch:   deploymentInfo.Commit.Branch,
			Root: deployment.Root{
				Name: deploymentInfo.Root.Name,
			},
			Repo: deployment.Repo{
				Owner: deploymentInfo.Repo.Owner,
				Name:  deploymentInfo.Repo.Name,
			},
		},
	}

	env.OnActivity(da.GithubCompareCommit, mock.Anything, compareCommitRequest).Return(compareCommitResponse, nil)
	env.OnActivity(da.StoreLatestDeployment, mock.Anything, storeDeploymentRequest).Return(nil)

	env.ExecuteWorkflow(testDeployerWorkflow, deployerRequest{
		Info:         deploymentInfo,
		LatestDeploy: latestDeployedRevision,
		ErrType:      TerraformClientError,
	})

	env.AssertExpectations(t)

	err := env.GetWorkflowError()

	wfErr, ok := err.(*temporal.WorkflowExecutionError)
	assert.True(t, ok)

	appErr, ok := wfErr.Unwrap().(*temporal.ApplicationError)
	assert.True(t, ok)

	assert.Equal(t, "TerraformClientError", appErr.Type())
}

func TestDeployer_Executor(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.OnGetVersion(version.SetPRRevision, workflow.DefaultVersion, 2).Return(workflow.Version(1))

	da := &testDeployActivity{}
	env.RegisterActivity(da)

	repo := github.Repo{
		Owner: "owner",
		Name:  "test",
	}

	root := model.Root{
		Name: "root_1",
	}

	deploymentInfo := terraform.DeploymentInfo{
		ID: uuid.UUID{},
		Commit: github.Commit{
			Revision: "3455",
			Branch:   "default-branch",
		},
		CheckRunID: 1234,
		Root:       root,
		Repo:       repo,
	}

	latestDeployedRevision := &deployment.Info{
		ID:       deploymentInfo.ID.String(),
		Version:  1.0,
		Revision: "3255",
		Branch:   "default-branch",
		Root: deployment.Root{
			Name: deploymentInfo.Root.Name,
		},
		Repo: deployment.Repo{
			Owner: deploymentInfo.Repo.Owner,
			Name:  deploymentInfo.Repo.Name,
		},
	}

	storeDeploymentRequest := activities.StoreLatestDeploymentRequest{
		DeploymentInfo: &deployment.Info{
			Version:  deployment.InfoSchemaVersion,
			ID:       deploymentInfo.ID.String(),
			Revision: deploymentInfo.Commit.Revision,
			Branch:   deploymentInfo.Commit.Branch,
			Root: deployment.Root{
				Name: deploymentInfo.Root.Name,
			},
			Repo: deployment.Repo{
				Owner: deploymentInfo.Repo.Owner,
				Name:  deploymentInfo.Repo.Name,
			},
		},
	}

	compareCommitRequest := activities.CompareCommitRequest{
		Repo:                   repo,
		DeployRequestRevision:  deploymentInfo.Commit.Revision,
		LatestDeployedRevision: latestDeployedRevision.Revision,
	}

	compareCommitResponse := activities.CompareCommitResponse{
		CommitComparison: activities.DirectionAhead,
	}

	env.OnActivity(da.GithubCompareCommit, mock.Anything, compareCommitRequest).Return(compareCommitResponse, nil)
	env.OnActivity(da.StoreLatestDeployment, mock.Anything, storeDeploymentRequest).Return(nil)

	env.ExecuteWorkflow(testDeployerWorkflow, deployerRequest{
		Info:         deploymentInfo,
		LatestDeploy: latestDeployedRevision,
	})

	env.AssertExpectations(t)

	var resp *deployResponse
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)

	assert.Equal(t, &deployment.Info{
		ID:       deploymentInfo.ID.String(),
		Version:  1.0,
		Revision: "3455",
		Branch:   "default-branch",
		Root: deployment.Root{
			Name: deploymentInfo.Root.Name,
		},
		Repo: deployment.Repo{
			Owner: deploymentInfo.Repo.Owner,
			Name:  deploymentInfo.Repo.Name,
		},
	}, resp.Info)
	assert.True(t, resp.ExecutorCalled)
}

func TestDeployer_SetPRRevision_NonDefaultBranchOld_v1(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.OnGetVersion(version.SetPRRevision, workflow.DefaultVersion, 2).Return(workflow.Version(1))

	da := &testDeployActivity{}
	env.RegisterActivity(da)

	repo := github.Repo{
		Owner:         "owner",
		Name:          "test",
		DefaultBranch: "main",
	}

	root := model.Root{
		Name: "root_1",
		TriggerInfo: model.TriggerInfo{
			Type: model.ManualTrigger,
		},
	}

	deploymentInfo := terraform.DeploymentInfo{
		ID: uuid.UUID{},
		Commit: github.Commit{
			Revision: "3455",
			Branch:   "test-branch",
		},
		CheckRunID: 1234,
		Root:       root,
		Repo:       repo,
	}

	latestDeployedRevision := &deployment.Info{
		ID:       deploymentInfo.ID.String(),
		Version:  1.0,
		Revision: "3255",
		Branch:   "main",
		Root: deployment.Root{
			Name: deploymentInfo.Root.Name,
		},
		Repo: deployment.Repo{
			Owner: deploymentInfo.Repo.Owner,
			Name:  deploymentInfo.Repo.Name,
		},
	}

	storeDeploymentRequest := activities.StoreLatestDeploymentRequest{
		DeploymentInfo: &deployment.Info{
			Version:  deployment.InfoSchemaVersion,
			ID:       deploymentInfo.ID.String(),
			Revision: deploymentInfo.Commit.Revision,
			Branch:   "test-branch",
			Root: deployment.Root{
				Name:    deploymentInfo.Root.Name,
				Trigger: "manual",
			},
			Repo: deployment.Repo{
				Owner: deploymentInfo.Repo.Owner,
				Name:  deploymentInfo.Repo.Name,
			},
		},
	}

	compareCommitRequest := activities.CompareCommitRequest{
		Repo:                   repo,
		DeployRequestRevision:  deploymentInfo.Commit.Revision,
		LatestDeployedRevision: latestDeployedRevision.Revision,
	}

	compareCommitResponse := activities.CompareCommitResponse{
		CommitComparison: activities.DirectionAhead,
	}

	env.RegisterWorkflow(testPRRevWorkflow)
	env.OnWorkflow(testPRRevWorkflow, mock.Anything, prrevision.Request{
		Repo:     repo,
		Root:     root,
		Revision: deploymentInfo.Commit.Revision,
	}).Return(nil)

	env.OnActivity(da.GithubCompareCommit, mock.Anything, compareCommitRequest).Return(compareCommitResponse, nil)
	env.OnActivity(da.StoreLatestDeployment, mock.Anything, storeDeploymentRequest).Return(nil)

	env.ExecuteWorkflow(testDeployerWorkflow, deployerRequest{
		Info:         deploymentInfo,
		LatestDeploy: latestDeployedRevision,
	})

	env.AssertExpectations(t)

	var resp *deployment.Info
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)

	assert.Equal(t, &deployment.Info{
		ID:       deploymentInfo.ID.String(),
		Version:  1.0,
		Revision: "3455",
		Branch:   "test-branch",
		Root: deployment.Root{
			Name:    deploymentInfo.Root.Name,
			Trigger: "manual",
		},
		Repo: deployment.Repo{
			Owner: deploymentInfo.Repo.Owner,
			Name:  deploymentInfo.Repo.Name,
		},
	}, resp)
}

func TestDeployer_SetPRRevision_NonDefaultBranchNew_v2(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	da := &testDeployActivity{}
	env.RegisterActivity(da)

	repo := github.Repo{
		Owner:         "owner",
		Name:          "test",
		DefaultBranch: "main",
	}

	root := model.Root{
		Name: "root_1",
		TriggerInfo: model.TriggerInfo{
			Type: model.ManualTrigger,
		},
	}

	deploymentInfo := terraform.DeploymentInfo{
		ID: uuid.UUID{},
		Commit: github.Commit{
			Revision: "3455",
			Branch:   "test-branch",
		},
		CheckRunID: 1234,
		Root:       root,
		Repo:       repo,
	}

	latestDeployedRevision := &deployment.Info{
		ID:       deploymentInfo.ID.String(),
		Version:  1.0,
		Revision: "3255",
		Branch:   "main",
		Root: deployment.Root{
			Name: deploymentInfo.Root.Name,
		},
		Repo: deployment.Repo{
			Owner: deploymentInfo.Repo.Owner,
			Name:  deploymentInfo.Repo.Name,
		},
	}

	storeDeploymentRequest := activities.StoreLatestDeploymentRequest{
		DeploymentInfo: &deployment.Info{
			Version:  deployment.InfoSchemaVersion,
			ID:       deploymentInfo.ID.String(),
			Revision: deploymentInfo.Commit.Revision,
			Branch:   "test-branch",
			Root: deployment.Root{
				Name:    deploymentInfo.Root.Name,
				Trigger: "manual",
			},
			Repo: deployment.Repo{
				Owner: deploymentInfo.Repo.Owner,
				Name:  deploymentInfo.Repo.Name,
			},
		},
	}

	compareCommitRequest := activities.CompareCommitRequest{
		Repo:                   repo,
		DeployRequestRevision:  deploymentInfo.Commit.Revision,
		LatestDeployedRevision: latestDeployedRevision.Revision,
	}

	compareCommitResponse := activities.CompareCommitResponse{
		CommitComparison: activities.DirectionAhead,
	}

	env.OnActivity(da.GithubCompareCommit, mock.Anything, compareCommitRequest).Return(compareCommitResponse, nil)
	env.OnActivity(da.StoreLatestDeployment, mock.Anything, storeDeploymentRequest).Return(nil)

	env.ExecuteWorkflow(testDeployerWorkflow, deployerRequest{
		Info:         deploymentInfo,
		LatestDeploy: latestDeployedRevision,
	})

	env.AssertExpectations(t)

	var resp *deployResponse
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)

	assert.Equal(t, &deployment.Info{
		ID:       deploymentInfo.ID.String(),
		Version:  1.0,
		Revision: "3455",
		Branch:   "test-branch",
		Root: deployment.Root{
			Name:    deploymentInfo.Root.Name,
			Trigger: "manual",
		},
		Repo: deployment.Repo{
			Owner: deploymentInfo.Repo.Owner,
			Name:  deploymentInfo.Repo.Name,
		},
	}, resp.Info)
}
