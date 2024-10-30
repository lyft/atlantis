package terraform

import (
	"reflect"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/notifier"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"github.com/runatlantis/atlantis/server/neptune/workflows/plugins"
	"go.temporal.io/sdk/workflow"
)

type WorkflowNotifier interface {
	Notify(workflow.Context, notifier.Info, *state.Workflow) error
}

type StateReceiver struct {

	// We have separate classes of notifiers since we can be more flexible with our internal ones in terms of the data model
	// What we support externally should be well thought out so for now this is kept to a minimum.
	Queue               deployQueue
	InternalNotifiers   []WorkflowNotifier
	AdditionalNotifiers []plugins.TerraformWorkflowNotifier
}

func (n *StateReceiver) Receive(ctx workflow.Context, c workflow.ReceiveChannel, deploymentInfo DeploymentInfo) {
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

	if workflowState.Apply.Status == state.WaitingJobStatus && reflect.ValueOf(workflowState.Apply.OnWaitingActions).IsZero() {
		// lock the queue item
	}

	for _, notifier := range n.InternalNotifiers {
		if err := notifier.Notify(ctx, deploymentInfo.ToInternalInfo(), workflowState); err != nil {
			workflow.GetMetricsHandler(ctx).Counter("notifier_failure").Inc(1)
			workflow.GetLogger(ctx).Error(errors.Wrap(err, "notifying workflow state change").Error())
		}
	}
}
