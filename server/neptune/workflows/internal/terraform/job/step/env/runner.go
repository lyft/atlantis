package env

import (
	"strings"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/job"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/steps"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/job/step/run"
)

type Runner struct {
	RunRunner run.Runner
}

func (e *Runner) Run(executionContext *job.ExecutionContext, rootInstance *steps.RootInstance, step steps.Step) (string, error) {
	if step.EnvVarValue != "" {
		return step.EnvVarValue, nil
	}

	res, err := e.RunRunner.Run(executionContext, rootInstance, step)
	// Trim newline from res to support running `echo env_value` which has
	// a newline. We don't recommend users run echo -n env_value to remove the
	// newline because -n doesn't work in the sh shell which is what we use
	// to run commands.
	return strings.TrimSuffix(res, "\n"), err
}
