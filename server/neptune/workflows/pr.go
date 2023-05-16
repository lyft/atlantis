package workflows

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/request"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision"
	"go.temporal.io/sdk/workflow"
)

var PRTaskQueue = pr.TaskQueue
var PRTerraformRevisionSignalID = revision.TerraformRevisionSignalID

const PRDestroyPlanMode = request.DestroyPlanMode
const PRNormalPlanMode = request.NormalPlanMode

type PRNewRevisionSignalRequest = revision.NewTerraformRevisionRequest
type PRRepo = request.Repo
type PRRoot = request.Root
type PRJob = request.Job
type PRStep = request.Step
type PRPlanMode = request.PlanMode
type PRAppCredentials = request.AppCredentials

type PRRequest = pr.Request

func PR(ctx workflow.Context, request PRRequest) error {
	return pr.Workflow(ctx, request, Terraform)
}
