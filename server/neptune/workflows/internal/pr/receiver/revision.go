package receiver

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/request"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/request/converter"
	workflowMetrics "github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	TerraformRevisionSignalID = "new-terraform-revision"
)

type RevisionReceiver struct {
	ctx   workflow.Context
	scope workflowMetrics.Scope
}

type NewTerraformCommitRequest struct {
	Repo     request.Repo
	Revision string
	Roots    []request.Root
}

type Revision struct {
	Repo     github.Repo
	Revision string
	Roots    []terraform.Root
}

func NewRevisionReceiver(ctx workflow.Context, scope workflowMetrics.Scope) RevisionReceiver {
	return RevisionReceiver{
		ctx:   ctx,
		scope: scope,
	}
}

func (r *RevisionReceiver) Receive(c workflow.ReceiveChannel, more bool) Revision {
	if !more {
		return Revision{}
	}

	ctx := workflow.WithRetryPolicy(r.ctx, temporal.RetryPolicy{
		MaximumAttempts: 5,
	})

	var request NewTerraformCommitRequest
	c.Receive(ctx, &request)

	repo := converter.Repo(request.Repo)
	var roots []terraform.Root
	for _, root := range request.Roots {
		roots = append(roots, converter.Root(root))
	}
	return Revision{
		Repo:     repo,
		Revision: request.Revision,
		Roots:    roots,
	}
}
