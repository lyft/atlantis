package job

import (
	"context"
	"sync"
)

type NoopStreamHandler struct {
	wg sync.WaitGroup
}

func (n *NoopStreamHandler) RegisterJob(id string) chan string {
	jobOutput := make(chan string)
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		for line := range jobOutput {
			n.handle(&OutputLine{
				JobID: id,
				Line:  line,
			})
		}
	}()

	return jobOutput
}

func (n *NoopStreamHandler) CloseJob(ctx context.Context, jobID string) error {
	return nil
}

func (n *NoopStreamHandler) CleanUp(ctx context.Context) error {
	n.wg.Wait()
	return nil
}

func (s *NoopStreamHandler) handle(_ *OutputLine) {
}
