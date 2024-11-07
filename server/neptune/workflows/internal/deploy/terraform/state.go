package terraform

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/notifier"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"github.com/runatlantis/atlantis/server/neptune/workflows/plugins"
	"go.temporal.io/sdk/workflow"
)

type WorkflowNotifier interface {
	Notify(workflow.Context, notifier.Info, *state.Workflow) error
}

type CheckRunClient interface {
	CreateOrUpdate(ctx workflow.Context, deploymentID string, request notifier.GithubCheckRunRequest) (int64, error)
}

type StateReceiver struct {

	// We have separate classes of notifiers since we can be more flexible with our internal ones in terms of the data model
	// What we support externally should be well thought out so for now this is kept to a minimum.
	Queue               deployQueue
	CheckRunCache       CheckRunClient
	InternalNotifiers   []WorkflowNotifier
	AdditionalNotifiers []plugins.TerraformWorkflowNotifier
}

func (n *StateReceiver) Receive(ctx workflow.Context, c workflow.ReceiveChannel, deploymentInfo DeploymentInfo) {
	// deploymentInfo is the current deployment being processed in TerraformWorkflow. Receive is triggered whenever the TerraformWorkflow has a state change.

	var workflowState *state.Workflow
	c.Receive(ctx, &workflowState)

	workflow.GetMetricsHandler(ctx).WithTags(map[string]string{
		metrics.SignalNameTag: state.WorkflowStateChangeSignal,
	}).Counter(metrics.SignalReceive).Inc(1)

	// for now we are doing these notifiers first because otherwise we'd need to version (since audit activities were moved here)
	// TODO: Add the version clause to clean this up
	for _, notifier := range n.AdditionalNotifiers {
		if err := notifier.Notify(ctx, deploymentInfo.ToExternalInfo(), workflowState.ToExternalWorkflowState()); err != nil {
			workflow.GetMetricsHandler(ctx).Counter("notifier_plugin_failure").Inc(1)
			workflow.GetLogger(ctx).Error(errors.Wrap(err, "notifying workflow state change").Error())
		}
	}
	// Updates github check run with Terraform statuses for the current running deployment
	// TODO: do not notify github if workflowState.Result.Status == InProgressWorkflowStatus && workflowState.Result.Reason == UnknownCompletionReason
	for _, notifier := range n.InternalNotifiers {
		if err := notifier.Notify(ctx, deploymentInfo.ToInternalInfo(), workflowState); err != nil {
			workflow.GetMetricsHandler(ctx).Counter("notifier_failure").Inc(1)
			workflow.GetLogger(ctx).Error(errors.Wrap(err, "notifying workflow state change").Error())
		}
	}

	// Updates all other deployments waiting in queue when the current deployment is pending a confirm/reject user action. Current deployment is not on the queue at this point since its child TerraformWorkflow was started.
	// CheckRunCache.CreateOrUpdate executes an activity and is a nondeterministic operation (i.e. it is not guaranteed to be executed in the same order across different workflow runs). This is why we need to check the workflow version to determine if we should update the check run.
	// See https://docs.temporal.io/develop/go/versioning#patching for how to upgrade workflow version.
	v := workflow.GetVersion(ctx, "SurfaceQueueInCheckRuns", workflow.DefaultVersion, 1)
	if v == workflow.DefaultVersion {
		return
	}
	if workflowState.Apply != nil &&
		len(workflowState.Apply.OnWaitingActions.Actions) > 0 {
		queuedDeployments := n.Queue.GetOrderedMergedItems()
		revisionsSummary := n.Queue.GetQueuedRevisionsSummary()
		var summary string

		if workflowState.Apply.Status == state.WaitingJobStatus {
			runLink := github.BuildRunURLMarkdown(deploymentInfo.Repo.GetFullName(), deploymentInfo.Commit.Revision, deploymentInfo.CheckRunID)
			summary = fmt.Sprintf("This deploy is queued pending action on run for revision %s.\n%s", runLink, revisionsSummary)
		} else if workflowState.Apply.Status == state.RejectedJobStatus || workflowState.Apply.Status == state.InProgressJobStatus {
			// If the current deployment is Rejected or In Progress status, we need to restore the queued check runs to reflect that the queued deployments are not blocked.
			summary = "This deploy is queued and will be processed as soon as possible.\n" + revisionsSummary
		}
		for _, i := range queuedDeployments {
			request := notifier.GithubCheckRunRequest{
				Title:   notifier.BuildDeployCheckRunTitle(i.Root.Name),
				Sha:     i.Commit.Revision,
				State:   github.CheckRunQueued,
				Repo:    i.Repo,
				Summary: summary,
			}

			workflow.GetLogger(ctx).Debug(fmt.Sprintf("Updating action pending summary for deployment id: %s", i.ID.String()))
			_, err := n.CheckRunCache.CreateOrUpdate(ctx, i.ID.String(), request)

			if err != nil {
				workflow.GetLogger(ctx).Debug(fmt.Sprintf("updating check run for revision %s", i.Commit.Revision), err)
			}
		}
	}
}
