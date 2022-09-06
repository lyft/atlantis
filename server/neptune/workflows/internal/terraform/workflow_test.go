package terraform_test

import (
	"context"
	"errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/root"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"testing"
	"time"
)

const (
	testRepoName     = "testrepo"
	testRootName     = "testroot"
	testDeploymentID = "123"
	testPath         = "rel/path"
)

type testActivities struct{}

func (a *testActivities) DownloadRoot(_ context.Context, _ activities.DownloadRootRequest) (activities.DownloadRootResponse, error) {
	return activities.DownloadRootResponse{}, nil
}

func testTerraformWorkflow(ctx workflow.Context) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: 5 * time.Second,
	})
	ch := workflow.NewChannel(ctx)
	act := &testActivities{}
	testRepo := github.Repo{
		Name: testRepoName,
	}
	testRoot := root.Root{
		Name: testRootName,
	}

	// Run download step
	var resp activities.DownloadRootResponse
	err := workflow.ExecuteActivity(ctx, act.DownloadRoot, activities.DownloadRootRequest{
		Repo:         testRepo,
		Root:         testRoot,
		DeploymentId: testDeploymentID,
	}).Get(ctx, &resp)
	if err != nil {
		return err
	}

	// TODO: run plan steps

	// Send plan approval signal
	approval := terraform.PlanReview{
		Status: terraform.Approved,
	}
	workflow.Go(ctx, func(ctx workflow.Context) {
		ch.Send(ctx, approval)
	})

	// Receive signal
	var planReview terraform.PlanReview
	ch.Receive(ctx, &planReview)
	if planReview.Status != terraform.Approved {
		return errors.New("failed to receive approval")
	}
	return nil

	// TODO: run apply steps
	// TODO: run cleanup step
}

func Test_TerraformWorkflowSuccess(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.SetWorkerOptions(worker.Options{
		BackgroundActivityContext: context.Background(),
	})
	a := &testActivities{}
	env.RegisterActivity(a)

	testRepo := github.Repo{
		Name: testRepoName,
	}
	testRoot := root.Root{
		Name: testRootName,
	}
	env.OnActivity(a.DownloadRoot, mock.Anything, activities.DownloadRootRequest{
		Repo:         testRepo,
		Root:         testRoot,
		DeploymentId: testDeploymentID,
	}).Return(activities.DownloadRootResponse{
		LocalRoot: &root.LocalRoot{
			Root: testRoot,
			Path: testPath,
			Repo: testRepo,
		},
	}, nil)

	env.ExecuteWorkflow(testTerraformWorkflow)
	env.AssertExpectations(t)
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())
}

func Test_TerraformWorkflow_CloneFailure(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.SetWorkerOptions(worker.Options{
		BackgroundActivityContext: context.Background(),
	})
	a := &testActivities{}
	env.RegisterActivity(a)

	testRepo := github.Repo{
		Name: testRepoName,
	}
	testRoot := root.Root{
		Name: testRootName,
	}
	env.OnActivity(a.DownloadRoot, mock.Anything, activities.DownloadRootRequest{
		Repo:         testRepo,
		Root:         testRoot,
		DeploymentId: testDeploymentID,
	}).Return(activities.DownloadRootResponse{}, errors.New("CloneActivityError"))

	env.ExecuteWorkflow(testTerraformWorkflow)
	assert.True(t, env.IsWorkflowCompleted())
	err := env.GetWorkflowError()
	var applicationErr *temporal.ApplicationError
	assert.True(t, errors.As(err, &applicationErr))
	assert.Equal(t, "CloneActivityError", applicationErr.Error())
}
