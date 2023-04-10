package terraform

import (
	"context"
	"strconv"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/metrics"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"go.temporal.io/sdk/workflow"
)

type WorkflowNotifier interface {
	Notify(workflow.Context, DeploymentInfo, *state.Workflow) error
}

type auditActivities interface {
	AuditJob(ctx context.Context, request activities.AuditJobRequest) error
}

type receiverActivities interface {
	auditActivities
	GithubUpdateCheckRun(ctx context.Context, request activities.UpdateCheckRunRequest) (activities.UpdateCheckRunResponse, error)
}

type StateReceiver struct {
	Activity  receiverActivities
	Notifiers []WorkflowNotifier
}

func (n *StateReceiver) Receive(ctx workflow.Context, c workflow.ReceiveChannel, deploymentInfo DeploymentInfo) {
	var workflowState *state.Workflow
	c.Receive(ctx, &workflowState)

	workflow.GetMetricsHandler(ctx).WithTags(map[string]string{
		metrics.SignalNameTag: state.WorkflowStateChangeSignal,
	}).Counter(metrics.SignalReceive).Inc(1)

	for _, notifier := range n.Notifiers {
		if err := notifier.Notify(ctx, deploymentInfo, workflowState); err != nil {
			workflow.GetLogger(ctx).Error(errors.Wrap(err, "notifying workflow state change").Error())
		}
	}
}

func (n *StateReceiver) emitApplyEvents(ctx workflow.Context, jobState *state.Job, deploymentInfo DeploymentInfo) error {
	var atlantisJobState activities.AtlantisJobState
	startTime := strconv.FormatInt(jobState.StartTime.Unix(), 10)

	var endTime string
	switch jobState.Status {
	case state.InProgressJobStatus:
		atlantisJobState = activities.AtlantisJobStateRunning
	case state.SuccessJobStatus:
		atlantisJobState = activities.AtlantisJobStateSuccess
		endTime = strconv.FormatInt(jobState.EndTime.Unix(), 10)
	case state.FailedJobStatus:
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
		IsForceApply:   deploymentInfo.Root.Trigger == terraform.ManualTrigger && !deploymentInfo.Root.Rerun,
	}

	return workflow.ExecuteActivity(ctx, n.Activity.AuditJob, auditJobReq).Get(ctx, nil)
}
