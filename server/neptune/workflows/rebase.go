package workflows

import (
	revisionsetter "github.com/runatlantis/atlantis/server/neptune/workflows/internal/revision_setter"
	"go.temporal.io/sdk/workflow"
)

type SetPRMinimumRevisionRequest = revisionsetter.Request

func SetPRMinimumRevision(ctx workflow.Context, request SetPRMinimumRevisionRequest) error {
	return revisionsetter.Workflow(ctx, request)
}
