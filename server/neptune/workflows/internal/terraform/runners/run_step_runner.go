package runners

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/steps"
	"go.temporal.io/sdk/workflow"
)

type ExecuteCommandActivities interface {
	ExecuteCommand(context.Context, activities.ExecuteCommandRequest) activities.ExecuteCommandResponse
}

const planfileSlashReplace = "::"

type RunStepRunner struct {
	Activity ExecuteCommandActivities
}

func GetPlanFilename(projName string) string {
	projName = strings.Replace(projName, "/", planfileSlashReplace, -1)
	return fmt.Sprintf("%s.tfplan", projName)
}

func GetShowResultFileName(projectName string) string {
	projName := strings.Replace(projectName, "/", planfileSlashReplace, -1)
	return fmt.Sprintf("%s-%s.json", projName)
}

func (r *RunStepRunner) Run(
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

	cmd := exec.Command("sh", "-c", step.RunCommand) // #nosec
	cmd.Dir = path

	baseEnvVars := os.Environ()
	customEnvVars := map[string]string{
		"REPO_NAME":    repo.Name,
		"REPO_OWNER":   repo.Owner,
		"DIR":          path,
		"HEAD_COMMIT":  commit.Ref,
		"PLANFILE":     filepath.Join(path, GetPlanFilename(projectName)),
		"SHOWFILE":     filepath.Join(path, GetShowResultFileName(projectName)),
		"PROJECT_NAME": projectName,
		"REPO_REL_DIR": repoRelDir,
		"USER_NAME":    commit.Author.Username,

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

	finalEnvVars := baseEnvVars
	for key, val := range customEnvVars {
		finalEnvVars = append(finalEnvVars, fmt.Sprintf("%s=%s", key, val))
	}
	for key, val := range envs {
		finalEnvVars = append(finalEnvVars, fmt.Sprintf("%s=%s", key, val))
	}

	var resp activities.ExecuteCommandResponse
	_ = workflow.ExecuteActivity(ctx, r.Activity.ExecuteCommand, activities.ExecuteCommandRequest{
		Step:             step,
		CustomEnvVars:    customEnvVars,
		Envs:             envs,
		Path:             path,
		TerraformVersion: tfVersion,
	}).Get(ctx, &resp)

	if resp.Error != nil {
		return "", resp.Error
	}

	return resp.Output, nil
}
