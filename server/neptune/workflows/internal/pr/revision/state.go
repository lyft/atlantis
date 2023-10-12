package revision

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/notifier"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"github.com/runatlantis/atlantis/server/neptune/workflows/plugins"
	"go.temporal.io/sdk/workflow"
)

const (
	CheckBeforeNotify = "checkbeforenotify"
)

type WorkflowNotifier interface {
	Notify(workflow.Context, notifier.Info, *state.Workflow) error
}

type StateReceiver struct {
	InternalNotifiers   []WorkflowNotifier
	AdditionalNotifiers []plugins.TerraformWorkflowNotifier
}

func (s *StateReceiver) Receive(ctx workflow.Context, c workflow.ReceiveChannel, roots map[string]RootInfo) {
	var workflowState *state.Workflow
	c.Receive(ctx, &workflowState)

	workflow.GetMetricsHandler(ctx).WithTags(map[string]string{
		metrics.SignalNameTag: state.WorkflowStateChangeSignal,
	}).Counter(metrics.SignalReceive).Inc(1)

	s.Notify(ctx, workflowState, roots)
}

func (s *StateReceiver) Notify(ctx workflow.Context, workflowState *state.Workflow, roots map[string]RootInfo) {
	rootInfo, ok := roots[workflowState.ID]

	// TODO remove versioning
	// there can be an edge case where we've canceled a previous check run to handle a new revision
	// but the canceled child terraform workflow has sent over one last update prior to shutdown
	// we should avoid notifying on this case
	v := workflow.GetVersion(ctx, CheckBeforeNotify, workflow.DefaultVersion, workflow.Version(1))
	if v != workflow.DefaultVersion && !ok {
		workflow.GetLogger(ctx).Warn(fmt.Sprintf("skipping notifying root %s", workflowState.ID))
		return
	}
	workflow.GetLogger(ctx).Info("receiving state change signal", "root", rootInfo.Root.Name)

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
