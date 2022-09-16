package job_test

import (
	"context"
	"testing"

	"github.com/runatlantis/atlantis/server/neptune/temporalworker/job"
	"github.com/stretchr/testify/assert"
)

func TestReceiverRegistry(t *testing.T) {
	jobID := "1234"
	outputMsg := "a"

	t.Run("adds a receiver and broadcast", func(t *testing.T) {
		recvRegistry := job.NewReceiverRegistry()

		ch := make(chan string)
		recvRegistry.AddReceiver(jobID, ch)

		go recvRegistry.Broadcast(job.OutputLine{
			JobID: jobID,
			Line:  outputMsg,
		})

		assert.Equal(t, outputMsg, <-ch)
	})

	t.Run("removes receiver when close", func(t *testing.T) {
		recvRegistry := job.NewReceiverRegistry()

		ch := make(chan string)
		recvRegistry.AddReceiver(jobID, ch)

		recvRegistry.Close(context.TODO(), jobID)

		for range ch {
		}
	})

	t.Run("removes receiver if blocking", func(t *testing.T) {
		recvRegistry := job.NewReceiverRegistry()

		ch := make(chan string)
		recvRegistry.AddReceiver(jobID, ch)

		// this call is blocking if receiver is not removed since we are not listening to it
		recvRegistry.Broadcast(job.OutputLine{
			JobID: jobID,
			Line:  outputMsg,
		})
	})
}
