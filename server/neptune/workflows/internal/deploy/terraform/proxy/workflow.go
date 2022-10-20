package proxy

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/deployment"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/config/logger"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	temporalInternal "github.com/runatlantis/atlantis/server/neptune/workflows/internal/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	QueueTerraformSignalName = "queue"
)

func Workflow(ctx workflow.Context, _ Request, child terraform.Workflow) error {
	runner := newRunner(ctx, child)

	// blocking
	return runner.Run(ctx)
}

type Runner struct {
	RevisionProcessor *RevisionProcessor
}

type Request struct{}

type QueueSignalRequest struct {
	Info terraform.DeploymentInfo
}

func newRunner(ctx workflow.Context, tfWorkflow terraform.Workflow) *Runner {
	var githubActivities *activities.Github
	var deployActivities *activities.Deploy

	return &Runner{
		//TODO
		RevisionProcessor: &RevisionProcessor{},
	}

}

func (r *Runner) Run(ctx workflow.Context) error {
	ch := workflow.GetSignalChannel(ctx, QueueTerraformSignalName)
	selector := workflow.NewSelector(ctx)

	var signalRequest QueueSignalRequest
	selector.AddReceive(ch, func(c workflow.ReceiveChannel, more bool) {
		c.Receive(ctx, &signalRequest)
	})

	var currentDeployment *deployment.Info
	for {
		selector.Select(ctx)

		deployment, err := r.RevisionProcessor.Process(ctx, signalRequest.Info, currentDeployment)
		if err != nil {
			logger.Error(ctx, "running tf workflow", "err", err)
		}

		currentDeployment = deployment

		if !selector.HasPending() {
			return nil
		}
	}
}
