package activities

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/execute"
)

type executeCommandActivities struct {
	// DefaultTFVersion is used for run steps of roots that don't pin a Terraform version,
	// the same as the terraform activities do.
	DefaultTFVersion *version.Version
}

type ExecuteCommandResponse struct {
	Output string
}

type ExecuteCommandRequest struct {
	Step           execute.Step
	Path           string
	DynamicEnvVars []EnvVar
	// TfVersion is the root's configured Terraform version. Empty means the default version.
	TfVersion string
}

func (t *executeCommandActivities) ExecuteCommand(ctx context.Context, request ExecuteCommandRequest) (ExecuteCommandResponse, error) {
	cmd := exec.Command("sh", "-c", request.Step.RunCommand) // #nosec
	cmd.Dir = request.Path

	finalEnvVars := os.Environ()

	// Expose the Terraform version this root will run with, as the terraform activities do, so
	// scripts (e.g. backend.sh) can gate behavior on it. Set before the request's env vars so
	// an explicit env step still wins.
	tfVersion, err := resolveTFVersion(request.TfVersion, t.DefaultTFVersion)
	if err != nil {
		return ExecuteCommandResponse{}, err
	}
	if tfVersion != nil {
		finalEnvVars = append(finalEnvVars, fmt.Sprintf("%s=%s", AtlantisTerraformVersion, tfVersion.String()))
	}

	requestEnvVars, err := getEnvs(request.DynamicEnvVars)
	if err != nil {
		return ExecuteCommandResponse{}, err
	}
	for key, val := range requestEnvVars {
		finalEnvVars = append(finalEnvVars, fmt.Sprintf("%s=%s", key, val))
	}

	cmd.Env = finalEnvVars
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ExecuteCommandResponse{}, errors.Wrapf(err, "running %q in %q: \n%s", request.Step.RunCommand, request.Path, out)
	}

	return ExecuteCommandResponse{
		Output: string(out),
	}, nil
}

// StringCommand is a command that outputs a string
type StringCommand struct {
	Command        string
	Dir            string
	AdditionalEnvs map[string]string
}

func (l *StringCommand) Load() (string, error) {
	cmd := exec.Command("sh", "-c", l.Command) // #nosec
	cmd.Dir = l.Dir

	finalEnvVars := os.Environ()
	for key, val := range l.AdditionalEnvs {
		finalEnvVars = append(finalEnvVars, fmt.Sprintf("%s=%s", key, val))
	}

	cmd.Env = finalEnvVars
	out, err := cmd.CombinedOutput()

	if err != nil {
		return "", errors.Wrap(err, "executing cmd")
	}

	// Trim newline from `out` to support running `echo env_value` which has
	// a newline. We don't recommend users run echo -n env_value to remove the
	// newline because -n doesn't work in the sh shell which is what we use
	// to run commands.
	return strings.TrimSuffix(string(out), "\n"), err
}

// EnvVar is a container used for activity requests
// and we use basic json serialization so we need to be explicit
// about our fields and can't do any clever interfacing here.
type EnvVar struct {
	Name    string
	Value   string
	Command StringCommand
}

func (v EnvVar) GetValue() (string, error) {
	if v.Value != "" {
		return v.Value, nil
	}

	return v.Command.Load()
}
