package notifier

import (
	"context"

	"github.com/runatlantis/atlantis/server/neptune/lyft/activities"
	t "github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/plugins"
	"go.temporal.io/sdk/workflow"

	"strconv"
)

type auditActivity interface {
	AuditJob(ctx context.Context, request activities.AuditJobRequest) error
}

type SNSNotifier struct {
	Activity auditActivity
}

func (n *SNSNotifier) Notify(ctx workflow.Context, deploymentInfo plugins.TerraformDeploymentInfo, workflowState *plugins.TerraformWorkflowState) error {
	if workflowState.Apply == nil {
		return nil
	}

	jobState := workflowState.Apply

	var atlantisJobState activities.AtlantisJobState
	startTime := strconv.FormatInt(jobState.StartTime.Unix(), 10)

	var endTime string
	switch jobState.Status {
	case plugins.InProgressJobStatus:
		atlantisJobState = activities.AtlantisJobStateRunning
	case plugins.SuccessJobStatus:
		atlantisJobState = activities.AtlantisJobStateSuccess
		endTime = strconv.FormatInt(jobState.EndTime.Unix(), 10)
	case plugins.FailedJobStatus:
		atlantisJobState = activities.AtlantisJobStateFailure
		endTime = strconv.FormatInt(jobState.EndTime.Unix(), 10)

	// no need to emit events on other states
	default:
		return nil
	}

	auditJobReq := activities.AuditJobRequest{
		Repo:           deploymentInfo.Repo,
		Root:           deploymentInfo.Root,
		JobID:          jobState.ID,
		InitiatingUser: deploymentInfo.InitiatingUser,
		Tags:           deploymentInfo.Tags,
		Revision:       deploymentInfo.Commit.Revision,
		State:          atlantisJobState,
		StartTime:      startTime,
		EndTime:        endTime,
		IsForceApply:   deploymentInfo.Root.Trigger == t.ManualTrigger && deploymentInfo.Root.Force,
	}

	return workflow.ExecuteActivity(ctx, n.Activity.AuditJob, auditJobReq).Get(ctx, nil)
}
