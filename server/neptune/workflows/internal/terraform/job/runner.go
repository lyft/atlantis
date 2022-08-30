package job

import (
	"strings"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/job"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/steps"
	"go.temporal.io/sdk/workflow"
)

// stepRunner runs individual run steps
type stepRunner interface {
	Run(executionContext *job.ExecutionContext, rootInstance *steps.RootInstance, step steps.Step) (string, error)
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
	terraformJob steps.Job,
	rootInstance *steps.RootInstance,
) (string, error) {
	jobExectionCtx := job.BuildExecutionContextFrom(ctx, *rootInstance, map[string]string{})
	var outputs []string

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
