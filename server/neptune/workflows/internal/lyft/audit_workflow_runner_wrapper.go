package lyft

import (
	"context"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities/deployment"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/config/logger"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision/queue"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"go.temporal.io/sdk/workflow"
)

type auditActivities interface {
	AuditJob(ctx context.Context, request activities.AuditJobRequest) error
}

type AuditWorkflowRunnerWrapper struct {
	Activity auditActivities
	queue.TerraformWorkflowRunner
}

func (w *AuditWorkflowRunnerWrapper) Run(ctx workflow.Context, deploymentInfo terraform.DeploymentInfo) error {
	if err := w.emit(ctx, activities.AtlantisJobStateRunning, deploymentInfo); err != nil {
		return errors.Wrap(err, "emitting atlantis job event")
	}

	if tfRunErr := w.TerraformWorkflowRunner.Run(ctx, deploymentInfo); tfRunErr != nil {
		if err := w.emit(ctx, activities.AtlantisJobStateFailure, deploymentInfo); err != nil {
			logger.Error(ctx, errors.Wrap(err, "failed to emit atlantis job event").Error())
		}
		return tfRunErr
	}

	if err := w.emit(ctx, activities.AtlantisJobStateSuccess, deploymentInfo); err != nil {
		logger.Error(ctx, errors.Wrap(err, "failed to emit atlantis job event").Error())
	}

	return nil
}

func (w *AuditWorkflowRunnerWrapper) emit(ctx workflow.Context, state activities.AtlantisJobState, deploymentInfo terraform.DeploymentInfo) error {
	err := workflow.ExecuteActivity(ctx, w.Activity.AuditJob, activities.AuditJobRequest{
		DeploymentInfo: deployment.Info{
			Version:    queue.DeploymentInfoVersion,
			ID:         deploymentInfo.ID.String(),
			CheckRunID: deploymentInfo.CheckRunID,
			Revision:   deploymentInfo.Revision,
			User:       deploymentInfo.User,
			Root:       deploymentInfo.Root,
			Repo:       deploymentInfo.Repo,
			Tags:       deploymentInfo.Tags,
		},
		State: state,
	}).Get(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "notifying deploy api")
	}
	return nil
}
