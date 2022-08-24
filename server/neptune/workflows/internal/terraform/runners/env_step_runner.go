package runners

import (
	"strings"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/steps"
)

type EnvStepRunner struct {
	RunStepRunner *RunStepRunner
}

func (e *EnvStepRunner) Run(executionContext steps.ExecutionContext, rootInstance *steps.RootInstance, step steps.Step) (string, error) {
	if step.EnvVarValue != "" {
		return step.EnvVarValue, nil
	}

	res, err := e.RunStepRunner.Run(executionContext, rootInstance, step)
	// Trim newline from res to support running `echo env_value` which has
	// a newline. We don't recommend users run echo -n env_value to remove the
	// newline because -n doesn't work in the sh shell which is what we use
	// to run commands.
	return strings.TrimSuffix(res, "\n"), err
}
