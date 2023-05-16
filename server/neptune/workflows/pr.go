package workflows

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision"
	"go.temporal.io/sdk/workflow"
)

var PRTaskQueue = pr.TaskQueue
var PRTerraformRevisionSignalID = revision.TerraformRevisionSignalID

type PRNewRevisionSignalRequest = revision.NewTerraformRevisionRequest

type PRRequest = pr.Request

func PR(ctx workflow.Context, request PRRequest) error {
	return pr.Workflow(ctx, request, Terraform)
}
