package workflows

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/proxy"
	"go.temporal.io/sdk/workflow"
)

type TerraformProxyQueueSignalRequest = proxy.QueueSignalRequest

type TerraformProxyRequest = proxy.Request

const TerraformProxyQueueSignalName = proxy.QueueTerraformSignalName

func TerraformProxy(ctx workflow.Context, request TerraformProxyRequest) error {
	return proxy.Workflow(ctx, request, Terraform)
}
