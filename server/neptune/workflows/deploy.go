package workflows

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/signals"
	"go.temporal.io/sdk/workflow"
)

// Export anything that callers need such as requests, signals, etc.
type DeployRequest = deploy.Request
type DeployNewRevisionSignal = signals.NewRevision
type DeployNewRevisionSignalRequest = signals.NewRevisionRequest

func Deploy(ctx workflow.Context, request DeployRequest) error {
	return deploy.Workflow(ctx, request)
}