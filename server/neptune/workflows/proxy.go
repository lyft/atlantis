package workflows

import (
	"go.temporal.io/sdk/workflow"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform/proxy"
)


type TerraformProxyQueueSignalRequest = proxy.QueueSignalRequest

type TerraformProxyRequest = proxy.Request
const TerraformProxyQueueSignalName = proxy.QueueTerraformSignalName


func TerraformProxy(ctx workflow.Context, request TerraformProxyRequest) {
	// TODO: change from nil
	proxy.Workflow(ctx, request, Terraform)
}