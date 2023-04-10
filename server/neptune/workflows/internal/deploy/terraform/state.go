package terraform

import (
	"context"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/metrics"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
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
