package activities

import (
	"context"

	"github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/core/terraform/helpers"
	"github.com/uber-go/tally/v4"
)

type TerraformExec interface {
	RunCommandAsync(ctx context.Context, jobID string, path string, args []string, customEnvVars map[string]string, v *version.Version) <-chan helpers.Line
}

type terraformActivities struct {
	TerraformExecutor TerraformExec
	DefaultTFVersion  *version.Version
	Scope             tally.Scope
}

// Terraform Init

type TerraformInitRequest struct {
}

func (t *terraformActivities) TerraformInit(ctx context.Context, request TerraformInitRequest) error {
	return nil
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
