package job

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/steps"
	"go.temporal.io/sdk/workflow"
)

// ExecutionContext wraps the workflow context with other info needed to execute a step
type ExecutionContext struct {
	Path      string
	Envs      map[string]string
	TfVersion string
	workflow.Context
}

func BuildExecutionContextFrom(ctx workflow.Context, rootInstance steps.RootInstance, envs map[string]string) *ExecutionContext {
	return &ExecutionContext{
		Context:   ctx,
		Path:      rootInstance.Root.Path,
		Envs:      envs,
		TfVersion: rootInstance.Root.TfVersion,
	}
}

type Job struct {
	Steps []Step
}
