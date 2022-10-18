package workflows

import (
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/request"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision/queue"
	"go.temporal.io/sdk/workflow"
)

// Export anything that callers need such as requests, signals, etc.
type DeployRequest = deploy.Request
type Repo = request.Repo
type Root = request.Root
type Job = request.Job
type Step = request.Step
type AppCredentials = request.AppCredentials
type Ref = request.Ref
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

var DeployNewRevisionSignalID = deploy.NewRevisionSignalID

type DeployActivities struct {
	activities.Deploy
}

func NewDeployActivities(deploymentCfg valid.StoreConfig, topicArn string) (*DeployActivities, error) {
	deployActivities, err := activities.NewDeploy(deploymentCfg, topicArn)

	if err != nil {
		return nil, errors.Wrap(err, "initializing deploy activities")
	}

	return &DeployActivities{
		Deploy: *deployActivities,
	}, nil
}

func Deploy(ctx workflow.Context, request DeployRequest) error {
	return deploy.Workflow(ctx, request, Terraform)
}
