package receiver

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/request"
	workflowMetrics "github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	ShutdownSignalID = "pr-close"
)

type ShutdownReceiver struct {
	ctx   workflow.Context
	scope workflowMetrics.Scope
}

// TODO: potentially remove
type NewShutdownRequest struct {
	Repo  request.Repo
	PRNum int
}

func NewShutdownReceiver(ctx workflow.Context, scope workflowMetrics.Scope) ShutdownReceiver {
	return ShutdownReceiver{
		ctx:   ctx,
		scope: scope,
	}
}

func (s *ShutdownReceiver) Receive(c workflow.ReceiveChannel, more bool) {
	if !more {
		return
	}

	ctx := workflow.WithRetryPolicy(s.ctx, temporal.RetryPolicy{
		MaximumAttempts: 5,
	})

	var request NewShutdownRequest
	c.Receive(ctx, &request)
	// logs
}
