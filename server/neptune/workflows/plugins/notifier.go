package plugins

import "go.temporal.io/sdk/workflow"

// TerraformWorkflowNotifiers get called whenever there is a change to the WorkflowState
// For example, when plans are in-progress or when they complete.
type TerraformWorkflowNotifier interface {
	Notify(workflow.Context, TerraformDeploymentInfo, *TerraformWorkflowState) error
}

// Customizable plugins for the deploy workflow
type Deploy struct {

	// A set of notifiers that are called for TerraformWorkflowState changes
	Notifiers []TerraformWorkflowNotifier
}
