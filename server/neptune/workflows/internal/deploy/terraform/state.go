package terraform

import (
	"context"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/config/logger"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github/markdown"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type receiverActivities interface {
	UpdateCheckRun(ctx context.Context, request activities.UpdateCheckRunRequest) (activities.UpdateCheckRunResponse, error)
	StoreLatestDeployment(ctx context.Context, request activities.StoreLatestDeploymentRequest) error
}

type StateReceiver struct {
	Repo     github.Repo
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

	summary := markdown.RenderWorkflowStateTmpl(workflowState)
	checkRunState := determineCheckRunState(workflowState)

	request := activities.UpdateCheckRunRequest{
		Title:   BuildCheckRunTitle(deploymentInfo.Root.Name),
		State:   checkRunState,
		Repo:    n.Repo,
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
	var resp activities.UpdateCheckRunResponse
	err := workflow.ExecuteActivity(ctx, n.Activity.UpdateCheckRun, request).Get(ctx, &resp)

	if err != nil {
		logger.Error(ctx, "updating check run", "err", err)
	}

	// Store deployment info if apply job is success
	if workflowState.Apply != nil && workflowState.Apply.Status == state.SuccessJobStatus {
		logger.Info(ctx, "storing latest deployment info")
		n.storeDeploymentInfo(ctx, deploymentInfo)
	}
}

func (n *StateReceiver) storeDeploymentInfo(ctx workflow.Context, deploymentInfo DeploymentInfo) error {
	// TODO: Call StoreDeploymentInfo and persist deployment info
	err := workflow.ExecuteActivity(ctx, n.Activity.StoreLatestDeployment, activities.StoreLatestDeploymentRequest{
		DeploymentInfo: activities.DeploymentInfo{
			ID:         deploymentInfo.ID.String(),
			CheckRunID: deploymentInfo.CheckRunID,
			Revision:   deploymentInfo.Revision,
			Root:       deploymentInfo.Root,
		},
		RepoName: n.Repo.Name,
	}).Get(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "persisting deployment info")
	}
	return nil
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
