package runners

import (
	"strings"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/steps"
)

// StepRunner runs custom run steps.
type StepRunner interface {
	Run(executionContext steps.ExecutionContext, rootInstance *steps.RootInstance, step steps.Step) (string, error)
}

// JobRunner runs a deploy plan/apply job
type JobRunner interface {
	Run(ctx steps.ExecutionContext, job steps.Job, rootInstance *steps.RootInstance) (string, error)
}

func NewJobRunner(runStepRunner StepRunner, envStepRunner StepRunner) *jobRunner {
	return &jobRunner{
		RunRunner: runStepRunner,
		EnvRunner: envStepRunner,
	}
}

func (r *jobRunner) Run(
	ctx steps.ExecutionContext,
	job steps.Job,
	rootInstance *steps.RootInstance,
) (string, error) {
	var outputs []string

	for _, step := range job.Steps {
		var out string
		var err error

		switch step.StepName {
		case "run":
			out, err = r.RunRunner.Run(ctx, rootInstance, step)
		case "env":
			out, err = r.EnvRunner.Run(ctx, rootInstance, step)
			(*ctx.Envs)[step.EnvVarName] = out
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

type jobRunner struct {
	EnvRunner StepRunner
	RunRunner StepRunner
}
