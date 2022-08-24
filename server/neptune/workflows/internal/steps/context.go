package steps

import "go.temporal.io/sdk/workflow"

// ExecutionContext wraps the workflow context with other info needed to execute a step
type ExecutionContext struct {
	Path      string
	Envs      *map[string]string
	TfVersion string
	workflow.Context
}

func BuildExecutionContextFrom(ctx workflow.Context, root Root, envs *map[string]string) *ExecutionContext {
	return &ExecutionContext{
		Context:   ctx,
		Path:      root.Path,
		Envs:      envs,
		TfVersion: root.TfVersion,
	}
}
