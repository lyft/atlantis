package runners

import (
	"context"
	"path/filepath"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/steps"
	"go.temporal.io/sdk/workflow"
)

type executeCommandActivities interface {
	ExecuteCommand(context.Context, activities.ExecuteCommandRequest) activities.ExecuteCommandResponse
}

type RunStepRunner struct {
	Activity executeCommandActivities
}

func (r *RunStepRunner) Run(executionContext steps.ExecutionContext, rootInstance *steps.RootInstance, step steps.Step) (string, error) {
	relPath, err := rootInstance.RelativePathFromRepo()
	if err != nil {
		return "", err
	}

	defaultEnvVars := map[string]string{
		"REPO_NAME":    rootInstance.Repo.Name,
		"REPO_OWNER":   rootInstance.Repo.Owner,
		"DIR":          executionContext.Path,
		"HEAD_COMMIT":  rootInstance.Repo.HeadCommit.Ref,
		"PLANFILE":     filepath.Join(executionContext.Path, rootInstance.GetPlanFilename()),
		"SHOWFILE":     filepath.Join(executionContext.Path, rootInstance.GetShowResultFileName()),
		"PROJECT_NAME": rootInstance.Name,
		"REPO_REL_DIR": relPath,
		"USER_NAME":    rootInstance.Repo.HeadCommit.Author.Username,

		// Set these 2 fields in the activity since it relies on machine specific configuration
		// "ATLANTIS_TERRAFORM_VERSION": tfVersion.String(),
		// "PATH":                       fmt.Sprintf("%s:%s", os.Getenv("PATH"), r.TerraformBinDir),

		// Not required when working from main branch
		// "WORKSPACE":                  jobContext.Workspace,
		// "PULL_AUTHOR":                request.Pull.Author,
		// "COMMENT_ARGS":               strings.Join(request.EscapedCommentArgs, ","),
		// "BASE_BRANCH_NAME":           request.Pull.BaseBranch,
		// "HEAD_BRANCH_NAME":           request.Pull.HeadBranch,
		// "HEAD_REPO_NAME":             request.HeadRepo.Name,
		// "HEAD_REPO_OWNER":            request.HeadRepo.Owner,
		// "PULL_NUM":                   fmt.Sprintf("%d", request.Pull.Num),
	}

	var resp activities.ExecuteCommandResponse
	_ = workflow.ExecuteActivity(executionContext.Context, r.Activity.ExecuteCommand, activities.ExecuteCommandRequest{
		Step:             step,
		DefaultEnvVars:   defaultEnvVars,
		CustomEnvVars:    *executionContext.Envs,
		Path:             executionContext.Path,
		TerraformVersion: executionContext.TfVersion,
	}).Get(executionContext, &resp)

	if resp.Error != nil {
		return "", resp.Error
	}

	return resp.Output, nil
}
