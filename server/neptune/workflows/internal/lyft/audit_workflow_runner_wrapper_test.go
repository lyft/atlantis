package lyft_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities/deployment"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision/queue"
	deploy_tf "github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/job"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/lyft"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/root"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type testWorkflowRunnerActivities struct{}

func (t *testWorkflowRunnerActivities) UpdateCheckRun(ctx context.Context, request activities.UpdateCheckRunRequest) (activities.UpdateCheckRunResponse, error) {
	return activities.UpdateCheckRunResponse{}, nil
}

func (t *testWorkflowRunnerActivities) AuditJob(ctx context.Context, request activities.AuditJobRequest) error {
	return nil
}

func successTfWorkflow(ctx workflow.Context, request terraform.Request) error {
	return nil
}

func failTfWorkflow(ctx workflow.Context, request terraform.Request) error {
	return errors.New("error")
}

type req struct {
	DeploymentInfo deploy_tf.DeploymentInfo
	Success        bool
}

func testWorkflow(ctx workflow.Context, request req) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: 5 * time.Second,
	})

	var ra *testWorkflowRunnerActivities
	var wfRunner *lyft.AuditWorkflowRunnerWrapper
	if request.Success {
		wfRunner = &lyft.AuditWorkflowRunnerWrapper{
			Activity:                ra,
			TerraformWorkflowRunner: deploy_tf.NewWorkflowRunner(ra, successTfWorkflow),
		}
	} else {
		wfRunner = &lyft.AuditWorkflowRunnerWrapper{
			Activity:                ra,
			TerraformWorkflowRunner: deploy_tf.NewWorkflowRunner(ra, failTfWorkflow),
		}
	}

	return wfRunner.Run(ctx, request.DeploymentInfo)
}

func TestWorkflowRunnerWrapper_Success(t *testing.T) {
	id := uuid.New()
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	ta := &testWorkflowRunnerActivities{}

	rt := root.Root{
		Name: "test-root",
	}

	repo := github.Repo{
		Owner: "owner",
		Name:  "test",
	}

	tags := map[string]string{
		activities.ProjectTagKey:     "test-project",
		activities.EnvironmentTagKey: "test-environment",
	}

	user := github.User{
		Username: "test-user",
	}

	env.RegisterWorkflow(testWorkflow)
	env.RegisterWorkflow(successTfWorkflow)
	env.OnActivity(ta.AuditJob, mock.Anything, activities.AuditJobRequest{
		DeploymentInfo: deployment.Info{
			Version:    queue.DeploymentInfoVersion,
			ID:         id.String(),
			CheckRunID: 1234,
			Revision:   "ff",
			User:       user,
			Root:       rt,
			Repo:       repo,
			Tags:       tags,
		},
		State: job.Running,
	}).Return(nil)

	env.OnActivity(ta.AuditJob, mock.Anything, activities.AuditJobRequest{
		DeploymentInfo: deployment.Info{
			Version:    queue.DeploymentInfoVersion,
			ID:         id.String(),
			CheckRunID: 1234,
			Revision:   "ff",
			User:       user,
			Root:       rt,
			Repo:       repo,
			Tags:       tags,
		},
		State: job.Success,
	}).Return(nil)

	env.ExecuteWorkflow(testWorkflow, req{
		DeploymentInfo: deploy_tf.DeploymentInfo{
			ID:         id,
			CheckRunID: 1234,
			Revision:   "ff",
			User:       user,
			Root:       rt,
			Repo:       repo,
			Tags:       tags,
		},
		Success: true,
	})
	err := env.GetWorkflowResult(nil)
	assert.NoError(t, err)

	env.AssertExpectations(t)
}

func TestWorkflowRunnerWrapper_Failure(t *testing.T) {
	id := uuid.New()
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	ta := &testWorkflowRunnerActivities{}

	rt := root.Root{
		Name: "test-root",
	}

	repo := github.Repo{
		Owner: "owner",
		Name:  "test",
	}

	tags := map[string]string{
		activities.ProjectTagKey:     "test-project",
		activities.EnvironmentTagKey: "test-environment",
	}

	user := github.User{
		Username: "test-user",
	}

	env.RegisterWorkflow(testWorkflow)
	env.RegisterWorkflow(failTfWorkflow)
	env.OnActivity(ta.AuditJob, mock.Anything, activities.AuditJobRequest{
		DeploymentInfo: deployment.Info{
			Version:    queue.DeploymentInfoVersion,
			ID:         id.String(),
			CheckRunID: 1234,
			Revision:   "ff",
			User:       user,
			Root:       rt,
			Repo:       repo,
			Tags:       tags,
		},
		State: job.Running,
	}).Return(nil)

	env.OnActivity(ta.AuditJob, mock.Anything, activities.AuditJobRequest{
		DeploymentInfo: deployment.Info{
			Version:    queue.DeploymentInfoVersion,
			ID:         id.String(),
			CheckRunID: 1234,
			Revision:   "ff",
			User:       user,
			Root:       rt,
			Repo:       repo,
			Tags:       tags,
		},
		State: job.Failure,
	}).Return(nil)

	env.ExecuteWorkflow(testWorkflow, req{
		DeploymentInfo: deploy_tf.DeploymentInfo{
			ID:         id,
			CheckRunID: 1234,
			Revision:   "ff",
			User:       user,
			Root:       rt,
			Repo:       repo,
			Tags:       tags,
		},
		Success: false,
	})
	err := env.GetWorkflowResult(nil)
	assert.ErrorContains(t, err, "error")

	env.AssertExpectations(t)
}
