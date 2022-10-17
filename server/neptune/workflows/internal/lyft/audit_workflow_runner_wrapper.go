package lyft

import (
	"context"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/config/logger"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision/queue"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/job"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/root"
	"go.temporal.io/sdk/workflow"
)

type auditActivities interface {
	AuditJob(ctx context.Context, request activities.AuditJobRequest) error
}

type workflowRunnerActivities interface {
	UpdateCheckRun(ctx context.Context, request activities.UpdateCheckRunRequest) (activities.UpdateCheckRunResponse, error)
	auditActivities
}

func NewWorkflowRunnerWithAuditiing(a workflowRunnerActivities, w terraform.Workflow) *WorkflowRunnerWrapper {
	return &WorkflowRunnerWrapper{
		Activity: a,
		TerraformWorkflowRunner: &terraform.WorkflowRunner{
			Workflow: w,
			StateReceiver: &terraform.StateReceiver{
				Activity: a,
			},
		},
	}
}

type WorkflowRunnerWrapper struct {
	Activity auditActivities
	queue.TerraformWorkflowRunner
}

func (w *WorkflowRunnerWrapper) Run(ctx workflow.Context, deploymentInfo terraform.DeploymentInfo) error {
	if err := w.emit(ctx, job.Running, deploymentInfo); err != nil {
		return errors.Wrap(err, "emitting atlantis job event")
	}

	result := w.TerraformWorkflowRunner.Run(ctx, deploymentInfo)

	if result != nil {
		if err := w.emit(ctx, job.Failure, deploymentInfo); err != nil {
			logger.Error(ctx, errors.Wrap(err, "failed to emit atlantis job event").Error())
		}
	} else {
		if err := w.emit(ctx, job.Success, deploymentInfo); err != nil {
			logger.Error(ctx, errors.Wrap(err, "failed to emit atlantis job event").Error())
		}
	}

	return result
}

func (w *WorkflowRunnerWrapper) emit(ctx workflow.Context, state job.State, deploymentInfo terraform.DeploymentInfo) error {
	err := workflow.ExecuteActivity(ctx, w.Activity.AuditJob, activities.AuditJobRequest{
		DeploymentInfo: root.DeploymentInfo{
			ID:         deploymentInfo.ID.String(),
			CheckRunID: deploymentInfo.CheckRunID,
			Revision:   deploymentInfo.Revision,
			User:       deploymentInfo.User,
			Root:       deploymentInfo.Root,
			Repo:       deploymentInfo.Repo,
		},
		State: state,
	}).Get(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "notifying deploy api")
	}
	return nil
}
