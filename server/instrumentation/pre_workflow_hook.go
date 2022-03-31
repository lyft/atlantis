package instrumentation

import (
	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/metrics"
)

type PreWorkflowHookRunner struct {
	events.PreWorkflowHooksCommandRunner
}

func (r *PreWorkflowHookRunner) RunPreHooks(ctx *command.Context) error {
	scope := ctx.Scope.SubScope("pre_workflow_hook")

	executionSuccess := scope.Counter(metrics.ExecutionSuccessMetric)
	executionError := scope.Counter(metrics.ExecutionErrorMetric)

	err := r.PreWorkflowHooksCommandRunner.RunPreHooks(ctx)
	if err != nil {
		executionError.Inc(1)
		return err
	}

	executionSuccess.Inc(1)
	return nil
}
