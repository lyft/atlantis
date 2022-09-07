package job

import (
	"go.temporal.io/sdk/workflow"
)

// ExecutionContext wraps the workflow context with other info needed to execute a step
type ExecutionContext struct {
	Path      string
	Envs      map[string]string
	TfVersion string
	JobID     string
	workflow.Context
}

type Job struct {
	Steps []Step
	ID    string
}
