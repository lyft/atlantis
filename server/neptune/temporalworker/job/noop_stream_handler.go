package job

import (
	"context"
)

type NoopStreamHandler struct{}

func (n *NoopStreamHandler) RegisterJob(id string) chan string {
	return make(chan string)
}

func (n *NoopStreamHandler) CloseJob(ctx context.Context, jobID string) error {
	return nil
}

func (n *NoopStreamHandler) CleanUp(ctx context.Context) error {
	return nil
}
