package revision

import (
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/notifier"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"github.com/runatlantis/atlantis/server/neptune/workflows/plugins"
	"go.temporal.io/sdk/workflow"
)

type WorkflowNotifier interface {
	Notify(workflow.Context, notifier.Info, *state.Workflow) error
}

type StateReceiver struct {
	InternalNotifiers   []WorkflowNotifier
	AdditionalNotifiers []plugins.TerraformWorkflowNotifier
	RootCache           map[string]RootInfo
}

func (s *StateReceiver) AddRoot(info RootInfo) {
	s.RootCache[info.ID.String()] = info
}

func (s *StateReceiver) Receive(ctx workflow.Context, c workflow.ReceiveChannel) {
	var workflowState *state.Workflow
	c.Receive(ctx, &workflowState)

	workflow.GetMetricsHandler(ctx).WithTags(map[string]string{
		metrics.SignalNameTag: state.WorkflowStateChangeSignal,
	}).Counter(metrics.SignalReceive).Inc(1)

	rootInfo := s.RootCache[workflowState.ID]
	for _, notifier := range s.InternalNotifiers {
		if err := notifier.Notify(ctx, rootInfo.ToInternalInfo(), workflowState); err != nil {
			workflow.GetMetricsHandler(ctx).Counter("notifier_failure").Inc(1)
			workflow.GetLogger(ctx).Error(errors.Wrap(err, "notifying workflow state change").Error())
		}
	}

	for _, notifier := range s.AdditionalNotifiers {
		if err := notifier.Notify(ctx, rootInfo.ToExternalInfo(), workflowState.ToExternalWorkflowState()); err != nil {
			workflow.GetMetricsHandler(ctx).Counter("notifier_plugin_failure").Inc(1)
			workflow.GetLogger(ctx).Error(errors.Wrap(err, "notifying workflow state change").Error())
		}
	}
}
