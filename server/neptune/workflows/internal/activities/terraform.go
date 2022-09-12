package activities

import (
	"context"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/terraform/ansi"
	"github.com/runatlantis/atlantis/server/neptune/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/job"
)

const DisableInputArg = "-input=false"

type TerraformClient interface {
	RunCommand(ctx context.Context, jobID string, path string, args []string, customEnvVars map[string]string, v *version.Version) <-chan terraform.Line
}

type terraformActivities struct {
	TerraformClient  TerraformClient
	DefaultTFVersion *version.Version
}

func NewTerraformActivities(client TerraformClient, defaultTfVersion *version.Version) *terraformActivities {
	return &terraformActivities{
		TerraformClient:  client,
		DefaultTFVersion: defaultTfVersion,
	}
}

// Terraform Init
type TerraformInitRequest struct {
	Step      job.Step
	Envs      map[string]string
	JobID     string
	TfVersion string
	Path      string
}

type TerraformInitResponse struct {
	Output string
}

func (t *terraformActivities) TerraformInit(ctx context.Context, request TerraformInitRequest) (TerraformInitResponse, error) {
	// Resolve the tf version to be used for this operation
	tfVersion, err := t.resolveVersion(request.TfVersion)
	if err != nil {
		return TerraformInitResponse{}, err
	}

	cmd, err := terraform.NewCommandArguments(
		terraform.Init,
		[]string{DisableInputArg},
		request.Step.ExtraArgs,
	)
	if err != nil {
		return TerraformInitResponse{}, errors.Wrap(err, "building command arguments")
	}

	ch := t.TerraformClient.RunCommand(ctx, request.JobID, request.Path, cmd.Build(), request.Envs, tfVersion)
	var lines []string
	for line := range ch {
		if line.Err != nil {
			err = errors.Wrap(line.Err, "executing command")
			break
		}
		lines = append(lines, line.Line)
	}
	output := strings.Join(lines, "\n")

	// sanitize output by stripping out any ansi characters.
	output = ansi.Strip(output)
	return TerraformInitResponse{}, nil
}

// Terraform Plan

type TerraformPlanRequest struct {
}

func (t *terraformActivities) TerraformPlan(ctx context.Context, request TerraformPlanRequest) error {
	return nil
}

// Terraform Apply

type TerraformApplyRequest struct {
}

func (t *terraformActivities) TerraformApply(ctx context.Context, request TerraformApplyRequest) error {
	return nil
}

func (t *terraformActivities) resolveVersion(v string) (*version.Version, error) {
	// Use default version if configured version is empty
	if v == "" {
		return t.DefaultTFVersion, nil
	}

	version, err := version.NewVersion(v)
	if err != nil {
		return nil, errors.Wrap(err, "resolving terraform version")
	}

	if version != nil {
		return version, nil
	}
	return t.DefaultTFVersion, nil
}
