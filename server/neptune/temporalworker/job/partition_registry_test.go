package job

import (
	"context"
	"sync"
	"testing"

	"github.com/runatlantis/atlantis/server/logging"
	"github.com/stretchr/testify/assert"
)

type testReceiverRegistry struct {
	t     *testing.T
	JobID string
	Ch    chan string
	Msg   OutputLine
}

func (t *testReceiverRegistry) AddReceiver(jobID string, ch chan string) {
	assert.Equal(t.t, t.JobID, jobID)
	assert.Equal(t.t, t.Ch, ch)
}

func (t *testReceiverRegistry) Broadcast(msg OutputLine) {
	assert.Equal(t.t, t.Msg, msg)
}

func (t *testReceiverRegistry) Close(ctx context.Context, jobID string) {
}

type strictTestReceiverRegistry struct {
	t           *testing.T
	addReceiver struct {
		runners []*testReceiverRegistry
		count   int
	}
	broadcast struct {
		runners []*testReceiverRegistry
		count   int
	}
	close struct {
		runners []*testReceiverRegistry
		count   int
	}
}

func (t strictTestReceiverRegistry) AddReceiver(jobID string, ch chan string) {
	if t.addReceiver.count > len(t.addReceiver.runners)-1 {
		t.t.FailNow()
	}
	t.addReceiver.runners[t.addReceiver.count].AddReceiver(jobID, ch)
	t.addReceiver.count += 1
	return
}

func (t strictTestReceiverRegistry) Broadcast(msg OutputLine) {
	if t.broadcast.count > len(t.broadcast.runners)-1 {
		t.t.FailNow()
	}
	t.broadcast.runners[t.broadcast.count].Broadcast(msg)
	t.broadcast.count += 1
	return
}

func (t strictTestReceiverRegistry) Close(ctx context.Context, jobID string) {
	if t.close.count > len(t.close.runners)-1 {
		t.t.FailNow()
	}
	t.close.runners[t.close.count].Close(ctx, jobID)
	t.close.count += 1
	return
}

type testStore struct {
	t      *testing.T
	JobID  string
	Output string
	Err    error
	Job    Job
	Status JobStatus
}

func (t *testStore) Get(jobID string) (*Job, error) {
	assert.Equal(t.t, t.JobID, jobID)
	return &t.Job, t.Err
}

func (t *testStore) Write(jobID string, output string) error {
	assert.Equal(t.t, t.JobID, jobID)
	assert.Equal(t.t, t.Output, output)
	return t.Err
}

func (t *testStore) Remove(jobID string) {
	assert.Equal(t.t, t.JobID, jobID)
}

func (t *testStore) Close(ctx context.Context, jobID string, status JobStatus) error {
	assert.Equal(t.t, t.JobID, jobID)
	assert.Equal(t.t, t.Status, status)
	return t.Err
}

type strictTestStore struct {
	t   *testing.T
	get struct {
		runners []*testStore
		count   int
	}
	write struct {
		runners []*testStore
		count   int
	}
	remove struct {
		runners []*testStore
		count   int
	}
	close struct {
		runners []*testStore
		count   int
	}
}

func (t strictTestStore) Get(jobID string) (*Job, error) {
	if t.get.count > len(t.get.runners)-1 {
		t.t.FailNow()
	}
	job, err := t.get.runners[t.get.count].Get(jobID)
	t.get.count += 1
	return job, err
}

func (t strictTestStore) Write(jobID string, output string) error {
	if t.write.count > len(t.write.runners)-1 {
		t.t.FailNow()
	}
	err := t.write.runners[t.write.count].Write(jobID, output)
	t.write.count += 1
	return err
}

func (t strictTestStore) Remove(jobID string) {
	if t.remove.count > len(t.remove.runners)-1 {
		t.t.FailNow()
	}
	t.remove.runners[t.remove.count].Remove(jobID)
	t.remove.count += 1
	return
}

func (t strictTestStore) Close(ctx context.Context, jobID string, status JobStatus) error {
	if t.close.count > len(t.close.runners)-1 {
		t.t.FailNow()
	}
	err := t.close.runners[t.close.count].Close(ctx, jobID, status)
	t.close.count += 1
	return err
}

func TestPartitionRegistry_Register(t *testing.T) {
	logs := []string{"a", "b"}
	jobID := "1234"

	t.Run("streams job output", func(t *testing.T) {
		testStore := &testStore{
			t:     t,
			JobID: jobID,
			Job: Job{
				Status: Complete,
				Output: logs,
			},
		}
		partitionRegistry := PartitionRegistry{
			ReceiverRegistry: &testReceiverRegistry{},
			Store:            testStore,
			Logger:           logging.NewNoopCtxLogger(t),
		}

		buffer := make(chan string, 100)
		go partitionRegistry.Register(jobID, buffer)

		receivedLogs := []string{}
		for line := range buffer {
			receivedLogs = append(receivedLogs, line)
		}

		assert.Equal(t, logs, receivedLogs)

	})

	t.Run("add to receiver registry when job is in progress", func(t *testing.T) {
		buffer := make(chan string)
		testStore := &strictTestStore{
			t: t,
			get: struct {
				runners []*testStore
				count   int
			}{
				runners: []*testStore{
					&testStore{
						t:     t,
						JobID: jobID,
						Job: Job{
							Status: Processing,
							Output: logs,
						},
					},
				},
			},
		}
		receiverRegistry := &strictTestReceiverRegistry{
			t: t,
			addReceiver: struct {
				runners []*testReceiverRegistry
				count   int
			}{
				runners: []*testReceiverRegistry{
					&testReceiverRegistry{
						t:     t,
						JobID: jobID,
						Ch:    buffer,
					},
				},
			},
		}

		partitionRegistry := PartitionRegistry{
			ReceiverRegistry: receiverRegistry,
			Store:            testStore,
			Logger:           logging.NewNoopCtxLogger(t),
		}

		go func() {
			for range buffer {
			}
		}()
		partitionRegistry.Register(jobID, buffer)
	})

	t.Run("closes receiver after streaming complete job", func(t *testing.T) {
		buffer := make(chan string)
		testStore := &strictTestStore{
			t: t,
			get: struct {
				runners []*testStore
				count   int
			}{
				runners: []*testStore{
					&testStore{
						t:     t,
						JobID: jobID,
						Job: Job{
							Status: Complete,
							Output: logs,
						},
					},
				},
			},
		}
		receiverRegistry := &strictTestReceiverRegistry{
			t: t,
			addReceiver: struct {
				runners []*testReceiverRegistry
				count   int
			}{
				runners: []*testReceiverRegistry{
					&testReceiverRegistry{
						t:     t,
						JobID: jobID,
						Ch:    buffer,
					},
				},
			},
		}

		partitionRegistry := PartitionRegistry{
			ReceiverRegistry: receiverRegistry,
			Store:            testStore,
			Logger:           logging.NewNoopCtxLogger(t),
		}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			for range buffer {
			}
			wg.Done()
		}()
		partitionRegistry.Register(jobID, buffer)

		// Read goroutine exits only when the buffer is closed
		wg.Wait()
	})
}
