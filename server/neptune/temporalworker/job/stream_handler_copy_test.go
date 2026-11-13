package job_test

import (
	"context"
	"errors"
	"testing"

	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/neptune/temporalworker/job"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type copyStore struct {
	jobs      map[string]*job.Job
	getErr    error
	writeErr  error
	writeLog  []string
	writeKeys []string
}

func (s *copyStore) Get(_ context.Context, jobID string) (*job.Job, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.jobs[jobID], nil
}

func (s *copyStore) Write(_ context.Context, jobID string, output string) error {
	if s.writeErr != nil {
		return s.writeErr
	}
	s.writeKeys = append(s.writeKeys, jobID)
	s.writeLog = append(s.writeLog, output)
	return nil
}

func (s *copyStore) Remove(string)                                      {}
func (s *copyStore) Close(context.Context, string, job.JobStatus) error { return nil }
func (s *copyStore) Cleanup(context.Context) error                      { return nil }

type copyRegistry struct {
	broadcast []job.OutputLine
}

func (r *copyRegistry) AddReceiver(string, chan string) {}
func (r *copyRegistry) Broadcast(msg job.OutputLine)    { r.broadcast = append(r.broadcast, msg) }
func (r *copyRegistry) CleanUp()                        {}
func (r *copyRegistry) Close(context.Context, string)   {}

func newCopyHandler(t *testing.T, store *copyStore, registry *copyRegistry) *job.StreamHandler {
	t.Helper()
	return job.NewStreamHandler(store, registry, valid.TerraformLogFilters{}, logging.NewNoopCtxLogger(t))
}

func TestStreamHandler_CopyJob(t *testing.T) {
	t.Run("writes the source output to the destination in order and broadcasts it", func(t *testing.T) {
		store := &copyStore{jobs: map[string]*job.Job{
			"plan": {Status: job.Complete, Output: []string{"plan 1", "plan 2"}},
		}}
		registry := &copyRegistry{}

		require.NoError(t, newCopyHandler(t, store, registry).CopyJob(context.Background(), "plan", "apply"))

		assert.Equal(t, []string{"plan 1", "plan 2"}, store.writeLog)
		assert.Equal(t, []string{"apply", "apply"}, store.writeKeys)
		assert.Equal(t, []job.OutputLine{{JobID: "apply", Line: "plan 1"}, {JobID: "apply", Line: "plan 2"}}, registry.broadcast)
	})

	t.Run("later apply output follows the copied plan", func(t *testing.T) {
		store := &copyStore{jobs: map[string]*job.Job{
			"plan": {Status: job.Complete, Output: []string{"plan 1"}},
		}}
		handler := newCopyHandler(t, store, &copyRegistry{})

		require.NoError(t, handler.CopyJob(context.Background(), "plan", "apply"))
		require.NoError(t, store.Write(context.Background(), "apply", "apply 1"))

		assert.Equal(t, []string{"plan 1", "apply 1"}, store.writeLog)
	})

	t.Run("a missing source copies nothing", func(t *testing.T) {
		store := &copyStore{jobs: map[string]*job.Job{}}
		registry := &copyRegistry{}

		require.NoError(t, newCopyHandler(t, store, registry).CopyJob(context.Background(), "plan", "apply"))

		assert.Empty(t, store.writeLog)
		assert.Empty(t, registry.broadcast)
	})

	t.Run("fails when the source can't be read", func(t *testing.T) {
		store := &copyStore{getErr: errors.New("boom")}

		assert.Error(t, newCopyHandler(t, store, &copyRegistry{}).CopyJob(context.Background(), "plan", "apply"))
	})

	t.Run("fails when the destination can't be written", func(t *testing.T) {
		store := &copyStore{
			jobs:     map[string]*job.Job{"plan": {Status: job.Complete, Output: []string{"plan 1"}}},
			writeErr: errors.New("boom"),
		}

		assert.Error(t, newCopyHandler(t, store, &copyRegistry{}).CopyJob(context.Background(), "plan", "apply"))
	})
}
