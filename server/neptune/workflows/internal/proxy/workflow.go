package proxy

import (
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/deployment"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"go.temporal.io/sdk/workflow"
)

const (
	QueueTerraformSignalName = "queue"
)

type proxyActivities struct {
	*activities.Github
	*activities.Deploy
}

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
		RevisionProcessor: &RevisionProcessor{
			Activities: &proxyActivities{
				Github: githubActivities,
				Deploy: deployActivities,
			},
			TerraformWorkflowRunner: terraform.NewWorkflowRunner(githubActivities, deployActivities, tfWorkflow),
		},
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
			return errors.Wrap(err, "processing revision")
		}

		currentDeployment = deployment

		if !selector.HasPending() {
			return nil
		}
	}
}
