package runners

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/core/runtime/cache"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/steps"
	"go.temporal.io/sdk/workflow"
)

type ExecuteCommandActivities interface {
	ExecuteCommand(context.Context, activities.ExecuteCommandRequest) error
}

const planfileSlashReplace = "::"

type RunStepRunner struct {
	versionCache cache.ExecutionVersionCache
	// Need a terraform client

	DefaultTFVersion *version.Version
	// TerraformBinDir is the directory where Atlantis downloads Terraform binaries.
	TerraformBinDir string

	activity ExecuteCommandActivities
}

// GetPlanFilename returns the filename (not the path) of the generated tf plan
// given a workspace and project name.
func GetPlanFilename(workspace string, projName string) string {
	if projName == "" {
		return fmt.Sprintf("%s.tfplan", workspace)
	}
	projName = strings.Replace(projName, "/", planfileSlashReplace, -1)
	return fmt.Sprintf("%s-%s.tfplan", projName, workspace)
}

// GetShowResultFileName returns the filename (not the path) to store the tf show result
func GetShowResultFileName(projectName string, workspace string) string {
	if projectName == "" {
		return fmt.Sprintf("%s.json", workspace)
	}
	projName := strings.Replace(projectName, "/", planfileSlashReplace, -1)
	return fmt.Sprintf("%s-%s.json", projName, workspace)
}

func (r *RunStepRunner) Run(ctx workflow.Context, step steps.Step, jobContext deploy.JobContext, envs map[string]string) (string, error) {

	tfVersion := r.DefaultTFVersion
	if jobContext.TerraformVersion != nil {
		tfVersion = jobContext.TerraformVersion
	}

	cmd := exec.Command("sh", "-c", step.RunCommand) // #nosec
	cmd.Dir = jobContext.Path

	baseEnvVars := os.Environ()
	customEnvVars := map[string]string{
		"ATLANTIS_TERRAFORM_VERSION": tfVersion.String(),
		"BASE_REPO_NAME":             jobContext.Repo.Name,
		"BASE_REPO_OWNER":            jobContext.Repo.Owner,
		"DIR":                        jobContext.Path,
		"HEAD_COMMIT":                jobContext.Commit.Ref,
		"PATH":                       fmt.Sprintf("%s:%s", os.Getenv("PATH"), r.TerraformBinDir),
		"PLANFILE":                   filepath.Join(jobContext.Path, GetPlanFilename(jobContext.Workspace, jobContext.ProjectName)),
		"SHOWFILE":                   filepath.Join(jobContext.Path, GetShowResultFileName(jobContext.ProjectName, jobContext.Workspace)),
		"PROJECT_NAME":               jobContext.ProjectName,
		"REPO_REL_DIR":               jobContext.RepoRelDir,
		"USER_NAME":                  jobContext.Commit.Author.Username,
		"WORKSPACE":                  jobContext.Workspace,

		// Not required when working from main branch
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

	// Call activity

	var resp activities.ExecuteCommandResponse
	_ = workflow.ExecuteActivity(ctx, r.activity.ExecuteCommand, activities.ExecuteCommandRequest{
		Step:             step,
		CustomEnvVars:    customEnvVars,
		Envs:             envs,
		Path:             jobContext.Path,
		TerraformVersion: jobContext.TerraformVersion,
	}).Get(ctx, &resp)

	if resp.Error != nil {
		return "", resp.Error
	}

	return resp.Output, nil
}
