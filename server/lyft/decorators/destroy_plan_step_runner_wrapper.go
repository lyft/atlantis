package decorators

import (
	"github.com/runatlantis/atlantis/server/events/command"
)

const Deprecated = "deprecated"
const Destroy = "-destroy"

type StepRunner interface {
	// Run runs the step.
	Run(ctx command.ProjectContext, extraArgs []string, path string, envs map[string]string) (string, error)
}

type DestroyPlanStepRunnerWrapper struct {
	StepRunner
}

func (d *DestroyPlanStepRunnerWrapper) Run(ctx command.ProjectContext, extraArgs []string, path string, envs map[string]string) (string, error) {
	// DestroyPlan tag is true when the Terraform client should construct a destroy plan given a repo config.
	if ctx.Tags[Deprecated] == Destroy {
		extraArgs = append(extraArgs, Destroy)
	}
	return d.StepRunner.Run(ctx, extraArgs, path, envs)
}
