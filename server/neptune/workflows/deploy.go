package workflows

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform"
	"go.temporal.io/sdk/workflow"
)

// Export anything that callers need such as requests, signals, etc.
type DeployRequest = deploy.Request
type Repo = deploy.Repo
type Root = deploy.Root
type Job = deploy.Job
type Step = deploy.Step

type DeployNewRevisionSignalRequest = revision.NewRevisionRequest

var DeployTaskQueue = deploy.TaskQueue
var TerraformTaskQueue = terraform.TaskQueue

var DeployNewRevisionSignalID = deploy.NewRevisionSignalID

func Deploy(ctx workflow.Context, request DeployRequest) error {
	return deploy.Workflow(ctx, request)
}
