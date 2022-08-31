package terraform

import (
	"context"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/config/logger"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github/markdown"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type activity interface {
	UpdateCheckRun(ctx context.Context, request activities.UpdateCheckRunRequest) (activities.UpdateCheckRunResponse, error)
}

type StateReceiver struct {
	ctx      workflow.Context
	repo     github.Repo
	activity activity
}

func NewStateReceiver(ctx workflow.Context, repo github.Repo, a activity) *StateReceiver {
	return &StateReceiver{
		ctx:      ctx,
		repo:     repo,
		activity: a,
	}
}

func (n *StateReceiver) Receive(c workflow.ReceiveChannel, checkRunID int64) {
	var workflowState *state.Workflow
	c.Receive(n.ctx, &workflowState)

	// TODO: if we never created a check run, there was likely some issue, we should attempt to create it again.
	if checkRunID == 0 {
		logger.Error(n.ctx, "check run id is 0, skipping update of check run")
		return
	}

	// this shouldn't be possible
	if workflowState.Plan == nil {
		logger.Error(n.ctx, "Plan job state is nil, This is likely a bug. Unable to update checks")
		return
	}

	summary := markdown.RenderWorkflowStateTmpl(workflowState)
	checkRunState, checkRunConclusion := determineCheckRunStateAndConclusion(workflowState)

	// cap our retries for non-terminal states to allow for at least some progress
	ctx := n.ctx
	if checkRunState != github.CheckRunComplete {
		ctx = workflow.WithRetryPolicy(n.ctx, temporal.RetryPolicy{
			MaximumAttempts: 3,
		})
	}

	// TODO: should we block here? maybe we can just make this async
	var resp activities.UpdateCheckRunResponse
	err := workflow.ExecuteActivity(ctx, n.activity.UpdateCheckRun, activities.UpdateCheckRunRequest{
		Title:      "atlantis/deploy",
		State:      checkRunState,
		Repo:       n.repo,
		ID:         checkRunID,
		Summary:    summary,
		Conclusion: checkRunConclusion,
	}).Get(n.ctx, &resp)

	if err != nil {
		logger.Error(ctx, "updating check run", "err", err)
	}
}

func determineCheckRunStateAndConclusion(workflowState *state.Workflow) (github.CheckRunState, github.CheckRunConclusion) {
	// determine conclusion, we can just base this on the apply job state basically
	var checkRunState github.CheckRunState
	var checkRunConclusion github.CheckRunConclusion
	if workflowState.Apply == nil {
		checkRunState = github.CheckRunPending
		return checkRunState, checkRunConclusion
	}

	switch workflowState.Apply.Status {
	case state.InProgressJobStatus, state.RejectedJobStatus:
		checkRunState = github.CheckRunPending
	case state.SuccessJobStatus:
		checkRunState = github.CheckRunComplete
		checkRunConclusion = github.CheckRunSuccess
	case state.FailedJobStatus:
		checkRunState = github.CheckRunComplete
		checkRunConclusion = github.CheckRunFailure
	}

	return checkRunState, checkRunConclusion
}
