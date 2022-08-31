package activities

import (
	"context"

	"github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/core/runtime"
	"github.com/uber-go/tally/v4"
)

type terraformActivities struct {
	TerraformExecutor runtime.TerraformExec
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
