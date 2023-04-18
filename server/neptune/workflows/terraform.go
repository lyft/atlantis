package workflows

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/gate"
	"go.temporal.io/sdk/workflow"
)

// Export anything that callers need such as requests, signals, etc.
type TerraformRequest = terraform.Request
type TerraformResponse = terraform.Response

type TerraformPlanReviewSignalRequest = gate.PlanReviewSignalRequest

type TerraformPlanReviewStatus = gate.PlanStatus

const ApprovedPlanReviewStatus = gate.Approved
const RejectedPlanReviewStatus = gate.Rejected

const TerraformPlanReviewSignalName = gate.PlanReviewSignalName

func Terraform(ctx workflow.Context, request TerraformRequest) (TerraformResponse, error) {
	return terraform.Workflow(ctx, request)
}
