package job

import (
	"context"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/execute"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"go.temporal.io/sdk/workflow"
)

type executeCommandActivities interface {
	ExecuteCommand(context.Context, activities.ExecuteCommandRequest) (activities.ExecuteCommandResponse, error)
}

type CmdStepRunner struct {
	Activity executeCommandActivities
}

func (r *CmdStepRunner) Run(executionContext *ExecutionContext, localRoot *terraform.LocalRoot, step execute.Step) (string, error) {
	relPath := localRoot.RelativePathFromRepo()

	envs := []EnvVar{
		NewEnvVarFromString("BASE_REPO_NAME", localRoot.Repo.Name),
		NewEnvVarFromString("BASE_REPO_OWNER", localRoot.Repo.Owner),
		NewEnvVarFromString("DIR", executionContext.Path),
		NewEnvVarFromString("PROJECT_NAME", localRoot.Root.Name),
		NewEnvVarFromString("REPO_REL_DIR", relPath),
	}
	envs = append(envs, executionContext.Envs...)

	var resp activities.ExecuteCommandResponse
	err := workflow.ExecuteActivity(executionContext.Context, r.Activity.ExecuteCommand, activities.ExecuteCommandRequest{
		Step:           step,
		Path:           executionContext.Path,
		DynamicEnvVars: toRequestEnvs(envs),
		EnvVars:        map[string]string{},
	}).Get(executionContext, &resp)
	if err != nil {
		return "", errors.Wrap(err, "executing activity")
	}

	return resp.Output, nil
}

func toRequestEnvs(envs []EnvVar) []activities.EnvVar {
	var result []activities.EnvVar
	for _, e := range envs {
		result = append(result, e.ToActivityEnvVar())
	}
	return result
}
