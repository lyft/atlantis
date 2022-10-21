package temporal

import (
	"github.com/pkg/errors"
	"go.temporal.io/sdk/workflow"
	"time"
)

func RecreateSession(ctx workflow.Context, err error) (workflow.Context, error) {
	sessionOptions := &workflow.SessionOptions{
		CreationTimeout:  time.Minute,
		ExecutionTimeout: 30 * time.Minute,
	}
	sessionInfo := workflow.GetSessionInfo(ctx)
	ctx, err = workflow.RecreateSession(ctx, sessionInfo.GetRecreateToken(), sessionOptions)
	if err != nil {
		return nil, errors.Wrap(err, "recreating session")
	}
	return ctx, nil
}
