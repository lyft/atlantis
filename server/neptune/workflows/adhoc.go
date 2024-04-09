package workflows

import (
	deploy "github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/request"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/request"
)

type AdhocRepo = request.Repo
type AdhocRoot = deploy.Root
type AdhocJob = request.Job

/*
type PRStep = request.Step
type PRPlanMode = request.PlanMode
type PRAppCredentials = request.AppCredentials
type PRRequest = pr.Request

func PR(ctx workflow.Context, request PRRequest) error {
	return pr.Workflow(ctx, request, Terraform)
}
*/
