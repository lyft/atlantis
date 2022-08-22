package runners

import (
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/steps"
	"go.temporal.io/sdk/workflow"
)

type EnvStepRunner struct {
	RunStepRunner *RunStepRunner
}

func (e *EnvStepRunner) Run(
	ctx workflow.Context,
	step steps.Step,
	repo github.Repo,
	commit github.Commit,
	tfVersion *version.Version,
	projectName string,
	repoRelDir string,
	path string,
	envs map[string]string,
) (string, error) {
	if step.EnvVarValue != "" {
		return step.EnvVarValue, nil
	}

	res, err := e.RunStepRunner.Run(ctx, step, repo, commit, tfVersion, projectName, repoRelDir, path, envs)
	// Trim newline from res to support running `echo env_value` which has
	// a newline. We don't recommend users run echo -n env_value to remove the
	// newline because -n doesn't work in the sh shell which is what we use
	// to run commands.
	return strings.TrimSuffix(res, "\n"), err
}
