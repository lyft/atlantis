package run

import (
	"context"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/job"
	"go.temporal.io/sdk/workflow"
)

type executeCommandActivities interface {
	ExecuteCommand(context.Context, activities.ExecuteCommandRequest) (activities.ExecuteCommandResponse, error)
}

type Runner struct {
	Activity executeCommandActivities
}

func (r *Runner) Run(executionContext *job.ExecutionContext, rootInstance *job.RootInstance, step job.Step) (string, error) {
	relPath, err := rootInstance.RelativePathFromRepo()
	if err != nil {
		return "", err
	}

	envVars := map[string]string{
		"REPO_NAME":    rootInstance.Repo.Name,
		"REPO_OWNER":   rootInstance.Repo.Owner,
		"DIR":          executionContext.Path,
		"HEAD_COMMIT":  rootInstance.Repo.HeadCommit.Ref,
		"PROJECT_NAME": rootInstance.Root.Name,
		"REPO_REL_DIR": relPath,
		"USER_NAME":    rootInstance.Repo.HeadCommit.Author.Username,
	}

	var resp activities.ExecuteCommandResponse
	err = workflow.ExecuteActivity(executionContext.Context, r.Activity.ExecuteCommand, activities.ExecuteCommandRequest{
		Step:    step,
		Path:    executionContext.Path,
		EnvVars: envVars,
	}).Get(executionContext, &resp)
	if err != nil {
		return "", errors.Wrap(err, "executing activity")
	}

	return resp.Output, nil
}
