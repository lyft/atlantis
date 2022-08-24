package activities

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/runtime/cache"
)

type executeCommandActivities struct {
	// Version Cache ensures the terraform version exists
	VersionCache cache.ExecutionVersionCache

	// DefaultTFVersion is the default configured version for atlantis is the tfVersion is not supplied
	DefaultTFVersion *version.Version

	// TerraformBinDir is the directory where Atlantis downloads Terraform binaries.
	TerraformBinDir string
}

type ExecuteCommandResponse struct {
	Output string
	Error  error
}

// Step was taken from the Atlantis OG config, we might be able to clean this up/remove it
type Step struct {
	StepName  string
	ExtraArgs []string
	// RunCommand is either a custom run step or the command to run
	// during an env step to populate the environment variable dynamically.
	RunCommand string
	// EnvVarName is the name of the
	// environment variable that should be set by this step.
	EnvVarName string
	// EnvVarValue is the value to set EnvVarName to.
	EnvVarValue string
}

type ExecuteCommandRequest struct {
	Step           Step
	Path           string
	CustomEnvVars  map[string]string
	DefaultEnvVars map[string]string

	// TerraformVersion is the version of terraform we should use when executing
	// commands for this project. This can be set to empty in which case we will
	// use the default Atlantis terraform version.
	TerraformVersion string
}

func (t *executeCommandActivities) ExecuteCommand(ctx context.Context, request ExecuteCommandRequest) ExecuteCommandResponse {

	var err error
	terraformVersion := t.DefaultTFVersion
	if request.TerraformVersion != "" {
		terraformVersion, err = version.NewVersion(request.TerraformVersion)
		if err != nil {
			return ExecuteCommandResponse{
				Output: "",
				Error:  errors.Wrapf(err, "parsing version %s", request.TerraformVersion),
			}
		}
	}

	// Ensure version exists
	_, err = t.VersionCache.Get(terraformVersion)
	if err != nil {
		return ExecuteCommandResponse{
			Output: "",
			Error:  errors.Wrapf(err, "getting version %s", request.TerraformVersion),
		}
	}

	// Need to set the ATLANTIS_TERRAFORM_VERSION and PATH here because we don't know the defaultTfVersion and the TerraformBinDir in the workflow
	terraformEnvVars := map[string]string{
		"ATLANTIS_TERRAFORM_VERSION": terraformVersion.String(),
		"PATH":                       fmt.Sprintf("%s:%s", os.Getenv("PATH"), t.TerraformBinDir),
	}

	cmd := exec.Command("sh", "-c", request.Step.RunCommand) // #nosec
	cmd.Dir = request.Path

	finalEnvVars := os.Environ()
	for key, val := range request.DefaultEnvVars {
		finalEnvVars = append(finalEnvVars, fmt.Sprintf("%s=%s", key, val))
	}
	for key, val := range request.CustomEnvVars {
		finalEnvVars = append(finalEnvVars, fmt.Sprintf("%s=%s", key, val))
	}
	for key, val := range terraformEnvVars {
		finalEnvVars = append(finalEnvVars, fmt.Sprintf("%s=%s", key, val))
	}

	cmd.Env = finalEnvVars
	out, err := cmd.CombinedOutput()

	if err != nil {
		return ExecuteCommandResponse{
			Output: "",
			Error:  fmt.Errorf("%s: running %q in %q: \n%s", err, request.Step.RunCommand, request.Path, out),
		}
	}
	return ExecuteCommandResponse{
		Output: string(out),
		Error:  nil,
	}
}
