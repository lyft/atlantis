package env_test

import (
	"testing"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/job"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/steps"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/job/step/env"
	"github.com/stretchr/testify/assert"
)

func TestEnvRunner(t *testing.T) {
	executioncontext := &job.ExecutionContext{}
	rootInstance := &steps.RootInstance{}

	step := steps.Step{
		EnvVarName:  "TEST_VAR",
		EnvVarValue: "TEST_VALUE",
	}

	t.Run("return env var value if set", func(t *testing.T) {
		runner := env.Runner{}

		out, err := runner.Run(executioncontext, rootInstance, step)
		assert.Nil(t, err)
		assert.Equal(t, out, step.EnvVarValue)
	})
}
