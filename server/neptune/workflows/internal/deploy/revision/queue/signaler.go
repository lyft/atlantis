package queue

import (
	"context"
	"fmt"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/proxy"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

type ProxyWorkflow func(workflow.Context, proxy.Request) error

type activity interface {
	SignalWithStartWorkflow(ctx context.Context, request activities.SignalWithStartWorkflowRequest) error
}

type ProxySignaler struct {
	ProxyWorkflow ProxyWorkflow
	Activity      activity
}

func (s ProxySignaler) SignalProxyWorkflow(ctx workflow.Context, msg terraform.DeploymentInfo) error {
	return workflow.ExecuteActivity(ctx, s.Activity.SignalWithStartWorkflow, activities.SignalWithStartWorkflowRequest{
		WorkflowID: fmt.Sprintf("%s||terraformproxy", workflow.GetInfo(ctx).WorkflowExecution.ID),
		SignalName: proxy.QueueTerraformSignalName,
		SignalArg: proxy.QueueSignalRequest{
			Info: msg,
		},
		Options:      client.StartWorkflowOptions{},
		Workflow:     s.ProxyWorkflow,
		WorkflowArgs: []interface{}{proxy.Request{}},
	}).Get(ctx, nil)
}
