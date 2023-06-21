package workflows

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/request"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision/policy"
	"go.temporal.io/sdk/workflow"
)

var PRTaskQueue = pr.TaskQueue
var PRTerraformRevisionSignalID = revision.TerraformRevisionSignalID
var PRShutdownSignalName = pr.ShutdownSignalID
var PRReviewSignalName = revision.ReviewSignalID

const PRDestroyPlanMode = request.DestroyPlanMode
const PRNormalPlanMode = request.NormalPlanMode

type PRShutdownRequest = pr.NewShutdownRequest
type PRNewRevisionSignalRequest = revision.NewTerraformRevisionRequest
type PRReviewRequest = policy.NewReviewRequest
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
