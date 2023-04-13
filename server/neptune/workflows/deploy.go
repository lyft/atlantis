package workflows

import (
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/request"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision/queue"
	"github.com/runatlantis/atlantis/server/neptune/workflows/plugins"
	"go.temporal.io/sdk/workflow"
)

// Export anything that callers need such as requests, signals, etc.
type DeployRequest = deploy.Request
type DeployRequestRepo = deploy.Repo
type DeployRequestRoot = deploy.Root
type Repo = request.Repo
type Root = request.Root
type Job = request.Job
type Step = request.Step
type AppCredentials = request.AppCredentials
type User = request.User
type PlanMode = request.PlanMode
type Trigger = request.Trigger

const DestroyPlanMode = request.DestroyPlanMode
const NormalPlanMode = request.NormalPlanMode

const ManualTrigger = request.ManualTrigger
const MergeTrigger = request.MergeTrigger

const DeployUnlockSignalName = queue.UnlockSignalName

type DeployUnlockSignalRequest = queue.UnlockSignalRequest
type DeployNewRevisionSignalRequest = revision.NewRevisionRequest

var DeployTaskQueue = deploy.TaskQueue
var DeployNewRevisionSignalID = revision.NewRevisionSignalID

// Workflow name
var Deploy = "Deploy"

// Workflow function is a closure, so make sure to register with a name
type DeployFunc func(workflow.Context, DeployRequest) error

// This is used to have user defined components of the workflow.
type InitDeployPlugins func(workflow.Context, DeployRequest) (*plugins.Deploy, error)

// NoPlugin is the default and should be used when there are no plugins to add
func NoPlugins(ctx workflow.Context, req DeployRequest) (*plugins.Deploy, error) {
	return nil, nil
}

func GetDeployWithPlugins(InitPlugins InitDeployPlugins) DeployFunc {
	return func(ctx workflow.Context, req DeployRequest) error {
		plugins, err := InitPlugins(ctx, req)
		if err != nil {
			return errors.Wrap(err, "building plugin")
		}
		return deploy.Workflow(
			ctx,
			req,
			deploy.ChildWorkflows{
				Terraform:     Terraform,
				SetPRRevision: PRRevision,
			},
			plugins,
		)
	}
}

func GetDeploy() DeployFunc {
	return GetDeployWithPlugins(NoPlugins)
}
