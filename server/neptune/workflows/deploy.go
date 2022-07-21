package workflows

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/signals"
	"go.temporal.io/sdk/workflow"
)

// Export anything that callers need such as requests, signals, etc.
type DeployRequest = deploy.Request
type DeployNewRevisionSignalRequest = signals.NewRevisionRequest

var DeployNewRevisionSignalID = signals.NewRevisionID

func Deploy(ctx workflow.Context, request DeployRequest) error {
	return deploy.Workflow(ctx, request)
}