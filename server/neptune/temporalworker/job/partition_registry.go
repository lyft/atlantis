package job

import (
	"context"
	"fmt"

	"github.com/runatlantis/atlantis/server/logging"
)

type PartitionRegistry struct {
	ReceiverRegistry receiverRegistry
	Store            Store
	Logger           logging.Logger
}

func (p PartitionRegistry) Register(ctx context.Context, key string, buffer chan string) {
	job, err := p.Store.Get(ctx, key)
	if err != nil || job == nil {
		p.Logger.Error(fmt.Sprintf("getting key partition: %s, err: %v", key, err))
		return
	}

	for _, line := range job.Output {
		buffer <- line
	}

	if job.Status == Complete {
		close(buffer)
		return
	}

	p.ReceiverRegistry.AddReceiver(key, buffer)
}
