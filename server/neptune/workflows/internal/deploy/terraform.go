package deploy

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform"
	"go.temporal.io/sdk/workflow"
)

// Export anything that callers need such as requests, signals, etc.
type TerraformRequest = terraform.Request

var TerraformTaskQueue = terraform.TaskQueue

// To be able to execute a child workflow within the deploy workflow and avoid import cycles, terraform workflow
// is defined within deploy package
func Terraform(ctx workflow.Context, request TerraformRequest) error {
	return terraform.Workflow(ctx, request)
}
