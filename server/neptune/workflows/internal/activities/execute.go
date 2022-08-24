package activities

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/runtime/cache"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/steps"
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

type ExecuteCommandRequest struct {
	Step           steps.Step
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

	// defaultTfVersion and the TerraformBinDir is only configured in the worker
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
