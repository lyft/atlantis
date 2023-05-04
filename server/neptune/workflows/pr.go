package workflows

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr"
	"go.temporal.io/sdk/workflow"
)

var PRTaskQueue = pr.TaskQueue

type PRRequest = pr.Request

func PR(ctx workflow.Context, request PRRequest) error {
	return pr.Workflow(ctx, request, Terraform)
}
