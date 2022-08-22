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
	versionCache cache.ExecutionVersionCache
}

type ExecuteCommandResponse struct {
	Output string
	Error  error
}

type ExecuteCommandRequest struct {
	Step          steps.Step
	CustomEnvVars map[string]string
	Envs          map[string]string

	Path string
	// TerraformVersion is the version of terraform we should use when executing
	// commands for this project. This can be set to nil in which case we will
	// use the default Atlantis terraform version.
	TerraformVersion *version.Version
}

func (t *executeCommandActivities) ExecuteCommand(ctx context.Context, request ExecuteCommandRequest) ExecuteCommandResponse {

	// Ensure version exists
	_, err := t.versionCache.Get(request.TerraformVersion)
	if err != nil {
		return ExecuteCommandResponse{
			Output: "",
			Error:  errors.Wrapf(err, "getting version %s", request.TerraformVersion),
		}
	}

	cmd := exec.Command("sh", "-c", request.Step.RunCommand) // #nosec
	cmd.Dir = request.Path

	baseEnvVars := os.Environ()

	finalEnvVars := baseEnvVars
	for key, val := range request.CustomEnvVars {
		finalEnvVars = append(finalEnvVars, fmt.Sprintf("%s=%s", key, val))
	}
	for key, val := range request.Envs {
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
