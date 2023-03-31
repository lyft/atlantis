package activities

import (
	"context"

	"github.com/pkg/errors"
	"go.temporal.io/sdk/activity"
)

type closer interface {
	CloseJob(ctx context.Context, jobID string) error
}

type jobActivities struct {
	StreamCloser closer
}

type CloseJobRequest struct {
	JobID string
}

func (t *jobActivities) CloseJob(ctx context.Context, request CloseJobRequest) error {
	err := t.StreamCloser.CloseJob(ctx, request.JobID)
	if err != nil {
		activity.GetLogger(ctx).Error(errors.Wrapf(err, "closing job").Error())
	}
	return err
}
