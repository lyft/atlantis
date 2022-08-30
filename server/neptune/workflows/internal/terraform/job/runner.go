package job

import (
	"strings"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/job"
	job_model "github.com/runatlantis/atlantis/server/neptune/workflows/internal/job"
	"go.temporal.io/sdk/workflow"
)

// stepRunner runs individual run steps
type stepRunner interface {
	Run(executionContext *job.ExecutionContext, rootInstance *job_model.RootInstance, step job_model.Step) (string, error)
}

type jobRunner struct {
	EnvStepRunner stepRunner
	RunStepRunner stepRunner
}

func NewRunner(runStepRunner stepRunner, envStepRunner stepRunner) *jobRunner {
	return &jobRunner{
		RunStepRunner: runStepRunner,
		EnvStepRunner: envStepRunner,
	}
}

func (r *jobRunner) Run(
	ctx workflow.Context,
	terraformJob job_model.Job,
	rootInstance *job_model.RootInstance,
) (string, error) {
	var outputs []string

	// Execution ctx for a job that handles setting up the env vars from the previous steps
	jobExectionCtx := job.BuildExecutionContextFrom(ctx, *rootInstance, map[string]string{})
	for _, step := range terraformJob.Steps {
		var out string
		var err error

		switch step.StepName {
		case "run":
			out, err = r.RunStepRunner.Run(jobExectionCtx, rootInstance, step)
		case "env":
			out, err = r.EnvStepRunner.Run(jobExectionCtx, rootInstance, step)
			jobExectionCtx.Envs[step.EnvVarName] = out
			// We reset out to the empty string because we don't want it to
			// be printed to the PR, it's solely to set the environment variable.
			out = ""
		}

		if out != "" {
			outputs = append(outputs, out)
		}
		if err != nil {
			return strings.Join(outputs, "\n"), err
		}
	}
	return strings.Join(outputs, "\n"), nil
}
