package activities

import (
	"context"
	"fmt"

	"github.com/runatlantis/atlantis/server/neptune/logger"
	"go.temporal.io/sdk/client"
)

type signaler interface {
	SignalWithStartWorkflow(ctx context.Context, workflowID string, signalName string, signalArg interface{},
		options client.StartWorkflowOptions, workflow interface{}, workflowArgs ...interface{}) (client.WorkflowRun, error)
}

type signalActivities struct {
	TemporalClient signaler
}

type SignalWithStartWorkflowRequest struct {
	WorkflowID   string
	SignalName   string
	SignalArg    interface{}
	Options      client.StartWorkflowOptions
	Workflow     interface{}
	WorkflowArgs []interface{}
}

func (a *signalActivities) SignalWithStartWorkflow(ctx context.Context, request SignalWithStartWorkflowRequest) error {
	run, err := a.TemporalClient.SignalWithStartWorkflow(
		ctx,
		request.WorkflowID,
		request.SignalName,
		request.SignalArg,
		request.Options,
		request.Workflow,
		request.WorkflowArgs...,
	)

	if err != nil {
		return err
	}

	logger.Info(ctx, fmt.Sprintf("signaled terraform proxy with id: %s", run.GetID()))

	return nil
}
