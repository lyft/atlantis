package runners

import (
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/steps"
	"go.temporal.io/sdk/workflow"
)

// CustomRunner runs custom run steps.
type StepRunner interface {
	Run(
		ctx workflow.Context,
		step steps.Step,
		repo github.Repo,
		commit github.Commit,
		tfVersion *version.Version,
		projectName string,
		repoRelDir string,
		path string,
		envs map[string]string,
	) (string, error)
}

// JobRunner runs a deploy job
type JobRunner interface {
	Run(
		ctx workflow.Context,
		job steps.Job,
		repo github.Repo,
		commit github.Commit,
		tfVersion *version.Version,
		projectName string,
		repoRelDir string,
		path string,
	) (string, error)
}

func NewJobRunner(
	runStepRunner StepRunner,
	envStepRunner StepRunner,
) JobRunner {
	jobRunner := &jobRunner{}
	jobRunner.RunRunner = runStepRunner
	jobRunner.EnvRunner = envStepRunner

	return jobRunner
}

func (r *jobRunner) Run(
	ctx workflow.Context,
	job steps.Job,
	repo github.Repo,
	commit github.Commit,
	tfVersion *version.Version,
	projectName string,
	repoRelDir string,
	path string,
) (string, error) {
	var outputs []string

	envs := make(map[string]string)
	for _, step := range job.Steps {
		var out string
		var err error
		switch step.StepName {
		case "init":
		case "plan":
		case "show":
		case "policy_check":
		case "apply":
		case "version":
		case "run":
			out, err = r.RunRunner.Run(ctx, step, repo, commit, tfVersion, projectName, repoRelDir, path, envs)
		case "env":
			out, err = r.EnvRunner.Run(ctx, step, repo, commit, tfVersion, projectName, repoRelDir, path, envs)
			envs[step.EnvVarName] = out
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
