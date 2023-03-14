package workflows

import (
	prrevision "github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr_revision"
	"go.temporal.io/sdk/workflow"
)

type PRRevisionRevisionRequest = prrevision.Request

func PRRevision(ctx workflow.Context, request PRRevisionRevisionRequest) error {
	return prrevision.Workflow(ctx, request)
}
