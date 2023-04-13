package plugins

import "go.temporal.io/sdk/workflow"

type TerraformWorkflowNotifier interface {
	Notify(workflow.Context, TerraformDeploymentInfo, *TerraformWorkflowState) error
}

type Deploy struct {
	Notifiers []TerraformWorkflowNotifier
}
