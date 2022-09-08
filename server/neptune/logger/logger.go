package logger

import (
	"context"

	internalContext "github.com/runatlantis/atlantis/server/neptune/context"
	"go.temporal.io/sdk/activity"
)

type Logger interface {
	Info(ctx context.Context, msg string)
	Warn(ctx context.Context, msg string)
	Error(ctx context.Context, msg string)
	Debug(ctx context.Context, msg string)
}

type ActivityLogger struct{}

func (d *ActivityLogger) Info(ctx context.Context, msg string) {
	logger := activity.GetLogger(ctx)
	kvs := internalContext.ExtractFieldsAsList(ctx)

	logger.Info(msg, kvs...)
}

func (d *ActivityLogger) Warn(ctx context.Context, msg string) {
	logger := activity.GetLogger(ctx)
	kvs := internalContext.ExtractFieldsAsList(ctx)

	logger.Warn(msg, kvs...)
}

func (d *ActivityLogger) Error(ctx context.Context, msg string) {
	logger := activity.GetLogger(ctx)
	kvs := internalContext.ExtractFieldsAsList(ctx)

	logger.Error(msg, kvs...)
}

func (d *ActivityLogger) Debug(ctx context.Context, msg string) {
	logger := activity.GetLogger(ctx)
	kvs := internalContext.ExtractFieldsAsList(ctx)

	logger.Debug(msg, kvs...)
}

type NoopLogger struct{}

func (d *NoopLogger) Info(ctx context.Context, msg string) {}

func (d *NoopLogger) Warn(ctx context.Context, msg string) {}

func (d *NoopLogger) Error(ctx context.Context, msg string) {}

func (d *NoopLogger) Debug(ctx context.Context, msg string) {}
