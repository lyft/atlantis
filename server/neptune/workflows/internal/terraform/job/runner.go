package job

import (
	"context"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/command"

	key "github.com/runatlantis/atlantis/server/neptune/context"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/execute"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	TerraformClientErrorType = "TerraformClientError"
)

// ExecutionContext wraps the workflow context with other info needed to execute a step
type ExecutionContext struct {
	Path      string
	Envs      []EnvVar
	TfVersion string
	workflow.Context
	JobID string
}

type terraformActivities interface {
	TerraformInit(ctx context.Context, request activities.TerraformInitRequest) (activities.TerraformInitResponse, error)
	TerraformPlan(ctx context.Context, request activities.TerraformPlanRequest) (activities.TerraformPlanResponse, error)
	TerraformApply(ctx context.Context, request activities.TerraformApplyRequest) (activities.TerraformApplyResponse, error)
	Conftest(ctx context.Context, request activities.ConftestRequest) (activities.ConftestResponse, error)
	CloseJob(ctx context.Context, request activities.CloseJobRequest) error
}

// stepRunner runs individual run steps
type stepRunner interface {
	Run(executionContext *ExecutionContext, localRoot *terraform.LocalRoot, step execute.Step) (string, error)
}

// envStepRunner computes env variables
type envStepRunner interface {
	Run(executionContext *ExecutionContext, localRoot *terraform.LocalRoot, step execute.Step) (EnvVar, error)
}

type JobRunner struct { ///nolint:revive // avoiding refactor while adding linter action
	Activity      terraformActivities
	EnvStepRunner envStepRunner
	CmdStepRunner stepRunner
}

func NewRunner(runStepRunner stepRunner, envStepRunner envStepRunner, tfActivities terraformActivities) *JobRunner {
	return &JobRunner{
		CmdStepRunner: runStepRunner,
		EnvStepRunner: envStepRunner,
		Activity:      tfActivities,
	}
}

func (r *JobRunner) Plan(ctx workflow.Context, localRoot *terraform.LocalRoot, jobID string, workflowMode terraform.WorkflowMode) (activities.TerraformPlanResponse, error) {
	ctx = workflow.WithRetryPolicy(ctx, temporal.RetryPolicy{
		NonRetryableErrorTypes: []string{TerraformClientErrorType},
	})
	// Execution ctx for a job that handles setting up the env vars from the previous steps
	jobCtx := &ExecutionContext{
		Context:   ctx,
		Path:      localRoot.Path,
		TfVersion: localRoot.Root.TfVersion,
		JobID:     jobID,
	}

	defer r.closeTerraformJob(jobCtx)

	var resp activities.TerraformPlanResponse

	for _, step := range localRoot.Root.Plan.GetSteps() {
		var err error
		switch step.StepName {
		case "init":
			err = r.init(jobCtx, localRoot, step)
		case "plan":
			resp, err = r.plan(jobCtx, localRoot.Root.Plan.Mode, workflowMode, step.ExtraArgs)
		}
		if err != nil {
			return resp, errors.Wrapf(err, "running step %s", step.StepName)
		}

		err = r.runOptionalSteps(jobCtx, localRoot, step)

		if err != nil {
			return resp, errors.Wrapf(err, "running step %s", step.StepName)
		}
	}

	return resp, nil
}

func (r *JobRunner) Validate(ctx workflow.Context, localRoot *terraform.LocalRoot, jobID string, showFile string) ([]activities.ValidationResult, error) {
	// Execution ctx for a job that handles setting up the env vars from the previous steps
	jobCtx := &ExecutionContext{
		Context: ctx,
		Path:    localRoot.Path,
		JobID:   jobID,
	}
	defer r.closeTerraformJob(jobCtx)

	var validateResults []activities.ValidationResult
	for _, step := range localRoot.Root.Validate.GetSteps() {
		var err error
		switch step.StepName {
		case "policy_check":
			validateResults, err = r.validate(jobCtx, showFile, step)
		}

		if err != nil {
			return validateResults, errors.Wrapf(err, "running step %s", step.StepName)
		}

		err = r.runOptionalSteps(jobCtx, localRoot, step)
		if err != nil {
			return validateResults, errors.Wrapf(err, "running step %s", step.StepName)
		}
	}
	return validateResults, nil
}

func (r *JobRunner) Apply(ctx workflow.Context, localRoot *terraform.LocalRoot, jobID string, planFile string) error {
	// Execution ctx for a job that handles setting up the env vars from the previous steps
	jobCtx := &ExecutionContext{
		Context:   ctx,
		Path:      localRoot.Path,
		TfVersion: localRoot.Root.TfVersion,
		JobID:     jobID,
	}
	defer r.closeTerraformJob(jobCtx)

	for _, step := range localRoot.Root.Apply.GetSteps() {
		var err error
		switch step.StepName {
		case "apply":
			err = r.apply(jobCtx, planFile, step)
		}

		if err != nil {
			return errors.Wrapf(err, "running step %s", step.StepName)
		}

		err = r.runOptionalSteps(jobCtx, localRoot, step)
		if err != nil {
			return errors.Wrapf(err, "running step %s", step.StepName)
		}
	}

	return nil
}

func (r *JobRunner) validate(executionCtx *ExecutionContext, showFile string, step execute.Step) ([]activities.ValidationResult, error) {
	args, err := command.NewArgumentList(step.ExtraArgs)
	if err != nil {
		return nil, errors.Wrapf(err, "creating argument list")
	}

	var envs []activities.EnvVar
	for _, e := range executionCtx.Envs {
		envs = append(envs, e.ToActivityEnvVar())
	}

	var resp activities.ConftestResponse
	err = workflow.ExecuteActivity(executionCtx, r.Activity.Conftest, activities.ConftestRequest{
		Args:        args,
		DynamicEnvs: envs,
		Path:        executionCtx.Path,
		JobID:       executionCtx.JobID,
		ShowFile:    showFile,
	}).Get(executionCtx, &resp)
	if err != nil {
		return resp.ValidationResults, errors.Wrap(err, "running conftest activity")
	}
	return resp.ValidationResults, nil
}

func (r *JobRunner) apply(executionCtx *ExecutionContext, planFile string, step execute.Step) error {
	args, err := command.NewArgumentList(step.ExtraArgs)

	if err != nil {
		return errors.Wrapf(err, "creating argument list")
	}

	var envs []activities.EnvVar
	for _, e := range executionCtx.Envs {
		envs = append(envs, e.ToActivityEnvVar())
	}
	var resp activities.TerraformApplyResponse

	// Applies should only be attempted once given there could be possible state mutations in the event
	// of heartbeat timeouts for example.
	ctx := workflow.WithRetryPolicy(executionCtx, temporal.RetryPolicy{
		MaximumAttempts: 1,
	})
	err = workflow.ExecuteActivity(ctx, r.Activity.TerraformApply, activities.TerraformApplyRequest{
		Args:        args,
		DynamicEnvs: envs,
		TfVersion:   executionCtx.TfVersion,
		Path:        executionCtx.Path,
		JobID:       executionCtx.JobID,
		PlanFile:    planFile,
	}).Get(ctx, &resp)

	if err != nil {
		return errors.Wrap(err, "running terraform apply activity")
	}
	return nil
}

func (r *JobRunner) plan(ctx *ExecutionContext, mode *terraform.PlanMode, workflowMode terraform.WorkflowMode, extraArgs []string) (activities.TerraformPlanResponse, error) {
	if workflowMode == terraform.Admin {
		// Admin mode doesn't need to run a plan.
		return activities.TerraformPlanResponse{}, nil
	}
	var resp activities.TerraformPlanResponse
	args, err := command.NewArgumentList(extraArgs)
	if err != nil {
		return resp, errors.Wrapf(err, "creating argument list")
	}
	var envs []activities.EnvVar
	for _, e := range ctx.Envs {
		envs = append(envs, e.ToActivityEnvVar())
	}
	err = workflow.ExecuteActivity(ctx, r.Activity.TerraformPlan, activities.TerraformPlanRequest{
		Args:         args,
		DynamicEnvs:  envs,
		TfVersion:    ctx.TfVersion,
		JobID:        ctx.JobID,
		Path:         ctx.Path,
		PlanMode:     mode,
		WorkflowMode: workflowMode,
	}).Get(ctx, &resp)
	if err != nil {
		return resp, errors.Wrap(err, "running terraform plan activity")
	}

	return resp, nil
}

func (r *JobRunner) init(ctx *ExecutionContext, localRoot *terraform.LocalRoot, step execute.Step) error {
	args, err := command.NewArgumentList(step.ExtraArgs)

	if err != nil {
		return errors.Wrap(err, "creating argument list")
	}
	var envs []activities.EnvVar
	for _, e := range ctx.Envs {
		envs = append(envs, e.ToActivityEnvVar())
	}
	var resp activities.TerraformInitResponse
	err = workflow.ExecuteActivity(ctx.Context, r.Activity.TerraformInit, activities.TerraformInitRequest{
		Args:                 args,
		DynamicEnvs:          envs,
		TfVersion:            ctx.TfVersion,
		Path:                 ctx.Path,
		JobID:                ctx.JobID,
		GithubInstallationID: localRoot.Repo.Credentials.InstallationToken,
	}).Get(ctx, &resp)
	if err != nil {
		return errors.Wrap(err, "running terraform init activity")
	}
	return nil
}

func (r *JobRunner) runOptionalSteps(ctx *ExecutionContext, localRoot *terraform.LocalRoot, step execute.Step) error {
	switch step.StepName {
	case "run":
		_, err := r.CmdStepRunner.Run(ctx, localRoot, step)
		return err
	case "env":
		o, err := r.EnvStepRunner.Run(ctx, localRoot, step)
		ctx.Envs = append(ctx.Envs, o)
		// output of env step doesn't need to be returned.
		return err
	}

	return nil
}

func (r *JobRunner) closeTerraformJob(executionCtx *ExecutionContext) {
	// create a new disconnected ctx since we want this run even in the event of
	// cancellation
	ctx := executionCtx.Context
	if temporal.IsCanceledError(executionCtx.Context.Err()) {
		var cancel workflow.CancelFunc
		ctx, cancel = workflow.NewDisconnectedContext(executionCtx.Context)
		defer cancel()
	}

	err := workflow.ExecuteActivity(ctx, r.Activity.CloseJob, activities.CloseJobRequest{
		JobID: executionCtx.JobID,
	}).Get(ctx, nil)

	if err != nil {
		workflow.GetLogger(ctx).Error("Error closing job", key.ErrKey, err)
	}
}
