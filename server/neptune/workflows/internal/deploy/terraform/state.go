package terraform

import (
	"context"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/deployment"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github/markdown"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/config/logger"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type auditActivities interface {
	AuditJob(ctx context.Context, request activities.AuditJobRequest) error
}

type receiverActivities interface {
	auditActivities
	UpdateCheckRun(ctx context.Context, request activities.UpdateCheckRunRequest) (activities.UpdateCheckRunResponse, error)
}

type StateReceiver struct {
	Activity receiverActivities
}

func (n *StateReceiver) Receive(ctx workflow.Context, c workflow.ReceiveChannel, deploymentInfo DeploymentInfo) {
	var workflowState *state.Workflow
	c.Receive(ctx, &workflowState)

	// TODO: if we never created a check run, there was likely some issue, we should attempt to create it again.
	if deploymentInfo.CheckRunID == 0 {
		logger.Error(ctx, "check run id is 0, skipping update of check run")
		return
	}

	// this shouldn't be possible
	if workflowState.Plan == nil {
		logger.Error(ctx, "Plan job state is nil, This is likely a bug. Unable to update checks")
		return
	}

	// emit audit events when Apply operation is run
	if workflowState.Apply != nil {
		if err := n.emitApplyEvents(ctx, workflowState.Apply.Status, deploymentInfo); err != nil {
			logger.Error(ctx, errors.Wrap(err, "auditing apply job event").Error())
		}
	}

	if err := n.updateCheckRun(ctx, workflowState, deploymentInfo); err != nil {
		logger.Error(ctx, "updating check run", "err", err)
	}
}

func (n *StateReceiver) updateCheckRun(ctx workflow.Context, workflowState *state.Workflow, deploymentInfo DeploymentInfo) error {
	summary := markdown.RenderWorkflowStateTmpl(workflowState)
	checkRunState := determineCheckRunState(workflowState)

	request := activities.UpdateCheckRunRequest{
		Title:   BuildCheckRunTitle(deploymentInfo.Root.Name),
		State:   checkRunState,
		Repo:    deploymentInfo.Repo,
		ID:      deploymentInfo.CheckRunID,
		Summary: summary,
	}

	if workflowState.Plan.Status == state.SuccessJobStatus &&
		workflowState.Apply != nil && workflowState.Apply.Status == state.WaitingJobStatus {
		request.Actions = []github.CheckRunAction{
			github.CreatePlanReviewAction(github.Approve),
			github.CreatePlanReviewAction(github.Reject),
		}
	}

	// cap our retries for non-terminal states to allow for at least some progress
	if checkRunState != github.CheckRunFailure && checkRunState != github.CheckRunSuccess {
		ctx = workflow.WithRetryPolicy(ctx, temporal.RetryPolicy{
			MaximumAttempts: 3,
		})
	}

	// TODO: should we block here? maybe we can just make this async
	return workflow.ExecuteActivity(ctx, n.Activity.UpdateCheckRun, request).Get(ctx, nil)
}

func (n *StateReceiver) emitApplyEvents(ctx workflow.Context, jobStatus state.JobStatus, deploymentInfo DeploymentInfo) error {
	var atlantisJobState activities.AtlantisJobState
	switch jobStatus {
	case state.InProgressJobStatus:
		atlantisJobState = activities.AtlantisJobStateRunning
	case state.SuccessJobStatus:
		atlantisJobState = activities.AtlantisJobStateSuccess
	case state.FailedJobStatus:
		atlantisJobState = activities.AtlantisJobStateFailure

	// no need to emit events on other states
	default:
		return nil
	}

	auditJobReq := activities.AuditJobRequest{
		DeploymentInfo: deployment.Info{
			Version:    deployment.DeploymentInfoVersion,
			ID:         deploymentInfo.ID.String(),
			CheckRunID: deploymentInfo.CheckRunID,
			Revision:   deploymentInfo.Revision,
			User:       deploymentInfo.User,
			Root:       deploymentInfo.Root,
			Repo:       deploymentInfo.Repo,
			Tags:       deploymentInfo.Tags,
		},
		State:        atlantisJobState,
		IsForceApply: deploymentInfo.Root.Trigger == terraform.ManualTrigger,
	}

	return workflow.ExecuteActivity(ctx, n.Activity.AuditJob, auditJobReq).Get(ctx, nil)
}

func determineCheckRunState(workflowState *state.Workflow) github.CheckRunState {
	if workflowState.Apply == nil {
		switch workflowState.Plan.Status {
		case state.InProgressJobStatus, state.SuccessJobStatus, state.WaitingJobStatus:
			return github.CheckRunPending
		case state.FailedJobStatus:
			return github.CheckRunFailure
		}
	}

	switch workflowState.Apply.Status {
	case state.InProgressJobStatus, state.WaitingJobStatus:
		return github.CheckRunPending
	case state.SuccessJobStatus:
		return github.CheckRunSuccess
	}

	// this is a failure or rejection at this point
	return github.CheckRunFailure
}
