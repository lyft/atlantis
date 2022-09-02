package workflows

import (
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/config"
	"github.com/runatlantis/atlantis/server/neptune/temporalworker/job"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform"
	"github.com/uber-go/tally/v4"
	"go.temporal.io/sdk/workflow"
)

// Export anything that callers need such as requests, signals, etc.
type TerraformRequest = terraform.Request

type TerraformActivities struct {
	activities.Terraform
}

func NewTerraformActivities(config config.TerraformConfig, scope tally.Scope, outputHandler *job.OutputHandler) (*TerraformActivities, error) {
	terraformActivities, err := activities.NewTerraform(config, scope, outputHandler)
	if err != nil {
		return nil, errors.Wrap(err, "initializing terraform activities")
	}
	return &TerraformActivities{
		Terraform: *terraformActivities,
	}, nil
}

func Terraform(ctx workflow.Context, request TerraformRequest) error {
	return terraform.Workflow(ctx, request)
}
