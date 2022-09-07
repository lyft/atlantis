package activities

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/terraform/helpers"
	"github.com/runatlantis/atlantis/server/events/runtime/common"
	"github.com/runatlantis/atlantis/server/events/terraform/ansi"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/job"
	"github.com/uber-go/tally/v4"
)

type terraformExec interface {
	RunCommandAsync(ctx context.Context, jobID string, path string, args []string, customEnvVars map[string]string, v *version.Version) <-chan helpers.Line
}

type terraformActivities struct {
	TerraformExecutor terraformExec
	DefaultTFVersion  *version.Version
	Scope             tally.Scope
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

	version, err := version.NewVersion(request.TfVersion)
	if err != nil {
		return TerraformInitResponse{}, errors.Wrap(err, "resolving terraform version")
	}

	tfVersion := t.DefaultTFVersion
	if version != nil {
		tfVersion = version
	}

	terraformInitVerb := []string{"init"}
	terraformInitArgs := []string{"-input=false"}
	finalArgs := common.DeDuplicateExtraArgs(terraformInitArgs, request.Step.ExtraArgs)
	terraformInitCmd := append(terraformInitVerb, finalArgs...)

	outCh := t.TerraformExecutor.RunCommandAsync(ctx, request.JobID, request.Path, terraformInitCmd, request.Envs, tfVersion)
	var lines []string
	for line := range outCh {
		if line.Err != nil {
			err = line.Err
			break
		}
		lines = append(lines, line.Line)
	}
	output := strings.Join(lines, "\n")

	// sanitize output by stripping out any ansi characters.
	output = ansi.Strip(output)
	return TerraformInitResponse{
		Output: fmt.Sprintf("%s\n", output),
	}, nil
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
