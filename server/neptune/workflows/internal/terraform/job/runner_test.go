package job_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	terraform_model "github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	job_model "github.com/runatlantis/atlantis/server/neptune/workflows/internal/job"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/root"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/job"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

const JobID = "1234"

type testTerraformActivity struct {
	t    *testing.T
	plan struct {
		req  activities.TerraformPlanRequest
		resp activities.TerraformPlanResponse
		err  error
	}
	apply struct {
		req  activities.TerraformApplyRequest
		resp activities.TerraformApplyResponse
		err  error
	}
	close struct {
		req  activities.TerraformCloseJobRequest
		resp activities.TerraformCloseJobResponse
		err  error
	}
}

func (t *testTerraformActivity) TerraformInit(ctx context.Context, request activities.TerraformInitRequest) (activities.TerraformInitResponse, error) {
	return activities.TerraformInitResponse{}, nil
}

func (t *testTerraformActivity) TerraformPlan(ctx context.Context, request activities.TerraformPlanRequest) (activities.TerraformPlanResponse, error) {
	assert.Equal(t.t, t.plan.req, request)
	return t.plan.resp, t.plan.err
}

func (t *testTerraformActivity) TerraformApply(ctx context.Context, request activities.TerraformApplyRequest) (activities.TerraformApplyResponse, error) {
	assert.Equal(t.t, t.apply.req, request)
	return t.apply.resp, t.apply.err
}

func (t *testTerraformActivity) TerraformCloseJob(ctx context.Context, request activities.TerraformCloseJobRequest) (activities.TerraformCloseJobResponse, error) {
	assert.Equal(t.t, t.close.req, request)
	return t.close.resp, t.close.err
}

func testJobWorkflow(ctx workflow.Context, r terraform.Request) (activities.TerraformPlanResponse, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: 100 * time.Second,
	})

	localRoot := root.LocalRoot{
		Root: r.Root,
		Repo: r.Repo,
		Path: ProjectPath,
	}

	jobExecutionCtx := &job_model.ExecutionContext{
		Context:   ctx,
		Path:      ProjectPath,
		Envs:      map[string]string{},
		TfVersion: localRoot.Root.TfVersion,
	}

	var a *testTerraformActivity
	jobRunner := job.NewRunner(&job.CmdStepRunner{}, &job.EnvStepRunner{}, a)
	return jobRunner.Plan(jobExecutionCtx, &localRoot, JobID)
}

func TestJobRunner_Plan(t *testing.T) {
	t.Run("should close job after plan operation", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()
		testTerraformActivity := &testTerraformActivity{
			t: t,
			plan: struct {
				req  activities.TerraformPlanRequest
				resp activities.TerraformPlanResponse
				err  error
			}{
				req: activities.TerraformPlanRequest{
					JobID: JobID,
					Args:  []terraform_model.Argument{},
					Envs:  map[string]string{},
					Path:  ProjectPath,
				},
			},
			close: struct {
				req  activities.TerraformCloseJobRequest
				resp activities.TerraformCloseJobResponse
				err  error
			}{
				req: activities.TerraformCloseJobRequest{
					JobID: JobID,
				},
			},
		}
		env.RegisterActivity(testTerraformActivity)
		env.RegisterWorkflow(testJobWorkflow)

		env.ExecuteWorkflow(testJobWorkflow, terraform.Request{
			Root: root.Root{
				Name: ProjectName,
				Path: "project",
				Plan: job_model.Job{
					Steps: []job_model.Step{
						{
							StepName: "plan",
						},
					},
				},
			},
			Repo: github.Repo{
				Name:  RepoName,
				Owner: RepoOwner,
				HeadCommit: github.Commit{
					Ref: github.Ref{
						Name: RefName,
						Type: RefType,
					},
					Author: github.User{
						Username: UserName,
					},
				},
			},
		})

		var resp activities.TerraformPlanResponse
		err := env.GetWorkflowResult(&resp)
		assert.NoError(t, err)
	})

	t.Run("should not fail plan operation when close job fails", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()
		testTerraformActivity := &testTerraformActivity{
			t: t,
			plan: struct {
				req  activities.TerraformPlanRequest
				resp activities.TerraformPlanResponse
				err  error
			}{
				req: activities.TerraformPlanRequest{
					JobID: JobID,
					Args:  []terraform_model.Argument{},
					Envs:  map[string]string{},
					Path:  ProjectPath,
				},
			},
			close: struct {
				req  activities.TerraformCloseJobRequest
				resp activities.TerraformCloseJobResponse
				err  error
			}{
				req: activities.TerraformCloseJobRequest{
					JobID: JobID,
				},
				err: errors.New("error"),
			},
		}
		env.RegisterActivity(testTerraformActivity)
		env.RegisterWorkflow(testJobWorkflow)

		env.ExecuteWorkflow(testJobWorkflow, terraform.Request{
			Root: root.Root{
				Name: ProjectName,
				Path: "project",
				Plan: job_model.Job{
					Steps: []job_model.Step{
						{
							StepName: "plan",
						},
					},
				},
			},
			Repo: github.Repo{
				Name:  RepoName,
				Owner: RepoOwner,
				HeadCommit: github.Commit{
					Ref: github.Ref{
						Name: RefName,
						Type: RefType,
					},
					Author: github.User{
						Username: UserName,
					},
				},
			},
		})

		var resp activities.TerraformPlanResponse
		err := env.GetWorkflowResult(&resp)
		assert.NoError(t, err)
	})
}

func TestJobRunner_Apply(t *testing.T) {
	t.Run("should close job after apply operation", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()
		testTerraformActivity := &testTerraformActivity{
			t: t,
			apply: struct {
				req  activities.TerraformApplyRequest
				resp activities.TerraformApplyResponse
				err  error
			}{
				req: activities.TerraformApplyRequest{
					JobID: JobID,
					Args:  []terraform_model.Argument{},
					Envs:  map[string]string{},
					Path:  ProjectPath,
				},
			},
			close: struct {
				req  activities.TerraformCloseJobRequest
				resp activities.TerraformCloseJobResponse
				err  error
			}{
				req: activities.TerraformCloseJobRequest{
					JobID: JobID,
				},
			},
		}
		env.RegisterActivity(testTerraformActivity)
		env.RegisterWorkflow(testJobWorkflow)

		env.ExecuteWorkflow(testJobWorkflow, terraform.Request{
			Root: root.Root{
				Name: ProjectName,
				Path: "project",
				Plan: job_model.Job{
					Steps: []job_model.Step{
						{
							StepName: "apply",
						},
					},
				},
			},
			Repo: github.Repo{
				Name:  RepoName,
				Owner: RepoOwner,
				HeadCommit: github.Commit{
					Ref: github.Ref{
						Name: RefName,
						Type: RefType,
					},
					Author: github.User{
						Username: UserName,
					},
				},
			},
		})

		var resp activities.TerraformPlanResponse
		err := env.GetWorkflowResult(&resp)
		assert.NoError(t, err)
	})

	t.Run("should not fail apply operation when close job fails", func(t *testing.T) {
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()
		testTerraformActivity := &testTerraformActivity{
			t: t,
			apply: struct {
				req  activities.TerraformApplyRequest
				resp activities.TerraformApplyResponse
				err  error
			}{
				req: activities.TerraformApplyRequest{
					JobID: JobID,
					Args:  []terraform_model.Argument{},
					Envs:  map[string]string{},
					Path:  ProjectPath,
				},
			},
			close: struct {
				req  activities.TerraformCloseJobRequest
				resp activities.TerraformCloseJobResponse
				err  error
			}{
				req: activities.TerraformCloseJobRequest{
					JobID: JobID,
				},
			},
		}
		env.RegisterActivity(testTerraformActivity)
		env.RegisterWorkflow(testJobWorkflow)

		env.ExecuteWorkflow(testJobWorkflow, terraform.Request{
			Root: root.Root{
				Name: ProjectName,
				Path: "project",
				Plan: job_model.Job{
					Steps: []job_model.Step{
						{
							StepName: "apply",
						},
					},
				},
			},
			Repo: github.Repo{
				Name:  RepoName,
				Owner: RepoOwner,
				HeadCommit: github.Commit{
					Ref: github.Ref{
						Name: RefName,
						Type: RefType,
					},
					Author: github.User{
						Username: UserName,
					},
				},
			},
		})

		var resp activities.TerraformPlanResponse
		err := env.GetWorkflowResult(&resp)
		assert.NoError(t, err)
	})
}
