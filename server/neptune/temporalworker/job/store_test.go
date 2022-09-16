package job_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/neptune/temporalworker/job"
	"github.com/stretchr/testify/assert"

	// . "github.com/petergtz/pegomock"
	. "github.com/runatlantis/atlantis/testing"
)

type testStorageBackend struct {
	t    *testing.T
	read struct {
		key  string
		resp []string
		err  error
	}
	write struct {
		key  string
		logs []string
		resp bool
		err  error
	}
}

type testStore struct {
	t      *testing.T
	JobID  string
	Output string
	Err    error
	Job    job.Job
}

func (t *testStore) Get(jobID string) (*job.Job, error) {
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

func (t *testStore) Close(ctx context.Context, jobID string) error {
	assert.Equal(t.t, t.JobID, jobID)
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

func (t strictTestStore) Get(jobID string) (*job.Job, error) {
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

func (t strictTestStore) Close(ctx context.Context, jobID string) error {
	if t.close.count > len(t.close.runners)-1 {
		t.t.FailNow()
	}
	err := t.close.runners[t.close.count].Close(ctx, jobID)
	t.close.count += 1
	return err
}

func (t *testStorageBackend) Read(key string) ([]string, error) {
	assert.Equal(t.t, t.read.key, key)
	return t.read.resp, t.read.err
}

func (t *testStorageBackend) Write(ctx context.Context, key string, logs []string) (bool, error) {
	assert.Equal(t.t, t.write.key, key)
	assert.Equal(t.t, t.write.logs, logs)
	return t.write.resp, t.write.err
}

func TestJobStore_Get(t *testing.T) {
	key := "1234"
	logs := []string{"a"}

	t.Run("load from memory", func(t *testing.T) {
		// Setup job store
		storageBackend := &testStorageBackend{}
		expectedJob := &job.Job{
			Output: logs,
			Status: job.Complete,
		}
		jobsMap := make(map[string]*job.Job)
		jobsMap[key] = expectedJob
		jobStore := job.NewTestJobStore(storageBackend, jobsMap)

		// Assert job
		gotJob, err := jobStore.Get(key)
		assert.NoError(t, err)
		assert.Equal(t, expectedJob.Output, gotJob.Output)
		assert.Equal(t, expectedJob.Status, gotJob.Status)
	})

	t.Run("load from storage backend when not in memory", func(t *testing.T) {
		// Setup job store
		expectedLogs := []string{"a", "b"}
		storageBackend := &testStorageBackend{
			t: t,
			read: struct {
				key  string
				resp []string
				err  error
			}{
				key:  key,
				resp: expectedLogs,
			},
		}
		expectedJob := job.Job{
			Output: expectedLogs,
			Status: job.Complete,
		}

		// Assert job
		jobStore := job.NewTestStorageBackedStore(logging.NewNoopCtxLogger(t), storageBackend, map[string]*job.Job{})
		gotJob, err := jobStore.Get(key)
		assert.NoError(t, err)
		assert.Equal(t, expectedJob.Output, gotJob.Output)
		assert.Equal(t, expectedJob.Status, gotJob.Status)
	})

	t.Run("error when reading from storage backend fails", func(t *testing.T) {
		// Setup job store
		expectedError := fmt.Errorf("reading from backend storage: error")
		storageBackend := &testStorageBackend{
			t: t,
			read: struct {
				key  string
				resp []string
				err  error
			}{
				key: key,
				err: errors.New("error"),
			},
		}

		// Assert job
		jobStore := job.NewTestStorageBackedStore(logging.NewNoopCtxLogger(t), storageBackend, map[string]*job.Job{})
		gotJob, err := jobStore.Get(key)
		assert.Empty(t, gotJob)
		assert.EqualError(t, expectedError, err.Error())
	})
}

func TestJobStore_Write(t *testing.T) {
	jobID := "1234"
	outpuMsg := "Test log message"

	t.Run("write new job", func(t *testing.T) {
		// Setup job store
		storageBackend := &testStorageBackend{}
		jobStore := job.NewTestStorageBackedStore(logging.NewNoopCtxLogger(t), storageBackend, map[string]*job.Job{})

		jobStore.Write(jobID, outpuMsg)

		// Assert job
		jb, err := jobStore.Get(jobID)
		Ok(t, err)
		assert.Equal(t, jb.Output, []string{outpuMsg})
		assert.Equal(t, jb.Status, job.Processing)
	})

	t.Run("write to existing job", func(t *testing.T) {
		// Setup job store
		storageBackend := &testStorageBackend{}
		jobStore := job.NewTestStorageBackedStore(logging.NewNoopCtxLogger(t), storageBackend, map[string]*job.Job{})
		output := []string{outpuMsg, outpuMsg}

		jobStore.Write(jobID, output[0])
		jobStore.Write(jobID, output[1])

		// Assert job
		jb, err := jobStore.Get(jobID)
		Ok(t, err)
		assert.Equal(t, jb.Output, output)
		assert.Equal(t, jb.Status, job.Processing)
	})

	t.Run("error when job status complete", func(t *testing.T) {
		// Setup job store
		jobsMap := map[string]*job.Job{
			jobID: &job.Job{
				Output: []string{outpuMsg},
				Status: job.Complete,
			},
		}
		storageBackend := &testStorageBackend{}
		jobStore := job.NewTestStorageBackedStore(logging.NewNoopCtxLogger(t), storageBackend, jobsMap)

		// Assert error
		err := jobStore.Write(jobID, "test message")
		assert.Error(t, err)
	})
}

func TestJobStore_Close(t *testing.T) {
	jobID := "1234"
	outputMsg := "a"

	t.Run("retain job in memory when persist fails", func(t *testing.T) {
		// Create new job and add it to store
		jobsMap := map[string]*job.Job{
			jobID: &job.Job{
				Output: []string{outputMsg},
				Status: job.Processing},
		}
		storageBackendErr := fmt.Errorf("random error")
		expecterErr := errors.Wrapf(storageBackendErr, "persisting job: %s", jobID)

		// Setup storage backend
		storageBackend := &testStorageBackend{
			t: t,
			write: struct {
				key  string
				logs []string
				resp bool
				err  error
			}{
				key:  jobID,
				logs: []string{outputMsg},
				resp: false,
				err:  storageBackendErr,
			},
		}
		jobStore := job.NewTestStorageBackedStore(logging.NewNoopCtxLogger(t), storageBackend, jobsMap)
		err := jobStore.Close(context.TODO(), jobID, job.Complete)

		// Assert storage backend error
		assert.EqualError(t, err, expecterErr.Error())

		// Assert the job is in memory
		jb, err := jobStore.Get(jobID)
		Ok(t, err)
		assert.Equal(t, jobsMap[jobID].Output, jb.Output)
		assert.Equal(t, jobsMap[jobID].Status, job.Complete)
	})

	t.Run("retain job in memory when storage backend not configured", func(t *testing.T) {
		// Create new job and add it to store
		jobsMap := map[string]*job.Job{
			jobID: &job.Job{
				Output: []string{outputMsg},
				Status: job.Processing,
			},
		}

		// Setup storage backend
		storageBackend := &job.NoopStorageBackend{}
		jobStore := job.NewTestStorageBackedStore(logging.NewNoopCtxLogger(t), storageBackend, jobsMap)
		err := jobStore.Close(context.TODO(), jobID, job.Complete)
		assert.Nil(t, err)

		// Assert the job is in memory
		jb, err := jobStore.Get(jobID)
		Ok(t, err)
		assert.Equal(t, jobsMap[jobID].Output, jb.Output)
		assert.Equal(t, jobsMap[jobID].Status, job.Complete)
	})

	t.Run("delete from memory when persist succeeds", func(t *testing.T) {
		// Create new job and add it to store
		jobsMap := map[string]*job.Job{
			jobID: &job.Job{
				Output: []string{outputMsg},
				Status: job.Processing,
			},
		}

		// Setup storage backend
		storageBackend := &testStorageBackend{
			t: t,
			write: struct {
				key  string
				logs []string
				resp bool
				err  error
			}{
				key:  jobID,
				logs: []string{outputMsg},
				resp: true,
				err:  nil,
			},
			read: struct {
				key  string
				resp []string
				err  error
			}{
				key: jobID,
			},
		}

		jobStore := job.NewTestStorageBackedStore(logging.NewNoopCtxLogger(t), storageBackend, jobsMap)
		err := jobStore.Close(context.TODO(), jobID, job.Complete)
		assert.Nil(t, err)

		gotJob, err := jobStore.Get(jobID)
		assert.Nil(t, err)
		assert.Empty(t, gotJob.Output)
	})

	t.Run("error when job does not exist", func(t *testing.T) {
		storageBackend := &testStorageBackend{}
		jobStore := job.NewTestStorageBackedStore(logging.NewNoopCtxLogger(t), storageBackend, map[string]*job.Job{})
		expectedErrString := fmt.Sprintf("job: %s does not exist", jobID)

		err := jobStore.Close(context.TODO(), jobID, job.Complete)
		assert.EqualError(t, err, expectedErrString)

	})
}
