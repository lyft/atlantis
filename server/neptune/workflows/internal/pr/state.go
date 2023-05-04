package pr

import (
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/notifier"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"go.temporal.io/sdk/workflow"
)

type WorkflowNotifier interface {
	Notify(workflow.Context, notifier.Info, *state.Workflow) error
}

type StateReceiver struct {
	InternalNotifiers []WorkflowNotifier
}

func (n *StateReceiver) Receive(ctx workflow.Context, c workflow.ReceiveChannel, prRootInfo PRRootInfo) {
	var workflowState *state.Workflow
	c.Receive(ctx, &workflowState)

	workflow.GetMetricsHandler(ctx).WithTags(map[string]string{
		metrics.SignalNameTag: state.WorkflowStateChangeSignal,
	}).Counter(metrics.SignalReceive).Inc(1)

	for _, notifier := range n.InternalNotifiers {
		if err := notifier.Notify(ctx, prRootInfo.ToInternalInfo(), workflowState); err != nil {
			workflow.GetMetricsHandler(ctx).Counter("notifier_failure").Inc(1)
			workflow.GetLogger(ctx).Error(errors.Wrap(err, "notifying workflow state change").Error())
		}
	}
}
