package plugins

import "go.temporal.io/sdk/workflow"

// TerraformWorkflowNotifiers get called whenever there is a change to the WorkflowState
// For example, when plans are in-progress or when they complete.
type TerraformWorkflowNotifier interface {
	Notify(workflow.Context, TerraformDeploymentInfo, *TerraformWorkflowState) error
}

// PostDeployExecutor can be used to enable specific actions to occur after a single deploy has been executed
type PostDeployExecutor interface {
	Execute(workflow.Context, TerraformDeploymentInfo) error
}

// Customizable plugins for the deploy workflow
type Deploy struct {

	// A set of notifiers that are called for TerraformWorkflowState changes
	Notifiers []TerraformWorkflowNotifier

	// A set of post deploy executors that are called after a deployment has transpired
	PostDeployExecutors []PostDeployExecutor
}
