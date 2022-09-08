package jobs_test

import (
	"fmt"
	"testing"

	"github.com/uber-go/tally/v4"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/jobs"
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

func (t *testStorageBackend) Read(key string) ([]string, error) {
	assert.Equal(t.t, t.read.key, key)
	return t.read.resp, t.read.err
}

func (t *testStorageBackend) Write(key string, logs []string) (bool, error) {
	assert.Equal(t.t, t.write.key, key)
	assert.Equal(t.t, t.write.logs, logs)
	return t.write.resp, t.write.err
}

func TestJobStore_Get(t *testing.T) {
	key := "1234"
	t.Run("load from memory", func(t *testing.T) {
		// Setup job store
		storageBackend := &testStorageBackend{}
		expectedJob := &jobs.Job{
			Output: []string{"a"},
			Status: jobs.Complete,
		}
		jobsMap := make(map[string]*jobs.Job)
		jobsMap[key] = expectedJob
		jobStore := jobs.NewTestJobStore(storageBackend, jobsMap)

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

		expectedJob := jobs.Job{
			Output: expectedLogs,
			Status: jobs.Complete,
		}

		// Assert job
		jobStore := jobs.NewJobStore(storageBackend, tally.NewTestScope("test", map[string]string{}))
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
		jobStore := jobs.NewJobStore(storageBackend, tally.NewTestScope("test", map[string]string{}))
		gotJob, err := jobStore.Get(key)
		assert.Empty(t, gotJob)
		assert.EqualError(t, expectedError, err.Error())
	})
}

func TestJobStore_AppendOutput(t *testing.T) {

	t.Run("append output when new job", func(t *testing.T) {
		// Setup job store
		storageBackend := &testStorageBackend{}
		jobStore := jobs.NewJobStore(storageBackend, tally.NewTestScope("test", map[string]string{}))
		jobID := "1234"
		output := "Test log message"

		jobStore.AppendOutput(jobID, output)

		// Assert job
		job, err := jobStore.Get(jobID)
		Ok(t, err)
		assert.Equal(t, job.Output, []string{output})
		assert.Equal(t, job.Status, jobs.Processing)
	})

	t.Run("append output when existing job", func(t *testing.T) {
		// Setup job store
		storageBackend := &testStorageBackend{}
		jobStore := jobs.NewJobStore(storageBackend, tally.NewTestScope("test", map[string]string{}))
		jobID := "1234"
		output := []string{"Test log message", "Test log message 2"}

		jobStore.AppendOutput(jobID, output[0])
		jobStore.AppendOutput(jobID, output[1])

		// Assert job
		job, err := jobStore.Get(jobID)
		Ok(t, err)
		assert.Equal(t, job.Output, output)
		assert.Equal(t, job.Status, jobs.Processing)
	})

	t.Run("error when job status complete", func(t *testing.T) {
		// Setup job store
		storageBackend := &testStorageBackend{}
		jobID := "1234"
		job := &jobs.Job{
			Output: []string{"a"},
			Status: jobs.Complete,
		}

		// Add complete to job in store
		jobsMap := make(map[string]*jobs.Job)
		jobsMap[jobID] = job
		jobStore := jobs.NewTestJobStore(storageBackend, jobsMap)

		// Assert error
		err := jobStore.AppendOutput(jobID, "test message")
		assert.Error(t, err)
	})
}

func TestJobStore_UpdateJobStatus(t *testing.T) {

	t.Run("retain job in memory when persist fails", func(t *testing.T) {
		// Create new job and add it to store
		jobID := "1234"
		logs := []string{"a"}
		job := &jobs.Job{
			Output: logs,
			Status: jobs.Processing,
		}
		jobsMap := make(map[string]*jobs.Job)
		jobsMap[jobID] = job
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
				logs: logs,
				resp: false,
				err:  storageBackendErr,
			},
		}
		// When(storageBackend.Write(AnyString(), matchers.AnySliceOfString())).ThenReturn(false, storageBackendErr)
		jobStore := jobs.NewTestJobStore(storageBackend, jobsMap)
		err := jobStore.SetJobCompleteStatus(jobID, jobs.Complete)

		// Assert storage backend error
		assert.EqualError(t, err, expecterErr.Error())

		// Assert the job is in memory
		jobInMem, err := jobStore.Get(jobID)
		Ok(t, err)
		assert.Equal(t, jobInMem.Output, job.Output)
		assert.Equal(t, job.Status, jobs.Complete)
	})

	t.Run("retain job in memory when storage backend not configured", func(t *testing.T) {
		// Create new job and add it to store
		jobID := "1234"
		job := &jobs.Job{
			Output: []string{"a"},
			Status: jobs.Processing,
		}
		jobsMap := make(map[string]*jobs.Job)
		jobsMap[jobID] = job

		// Setup storage backend
		storageBackend := &jobs.NoopStorageBackend{}
		jobStore := jobs.NewTestJobStore(storageBackend, jobsMap)
		err := jobStore.SetJobCompleteStatus(jobID, jobs.Complete)

		assert.Nil(t, err)

		// Assert the job is in memory
		jobInMem, err := jobStore.Get(jobID)
		Ok(t, err)
		assert.Equal(t, jobInMem.Output, job.Output)
		assert.Equal(t, job.Status, jobs.Complete)
	})

	t.Run("delete from memory when persist succeeds", func(t *testing.T) {
		// Create new job and add it to store
		jobID := "1234"
		logs := []string{"a"}
		job := &jobs.Job{
			Output: logs,
			Status: jobs.Processing,
		}
		jobsMap := make(map[string]*jobs.Job)
		jobsMap[jobID] = job

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
				logs: logs,
				resp: true,
			},

			read: struct {
				key  string
				resp []string
				err  error
			}{
				key:  jobID,
				resp: []string{},
			},
		}
		jobStore := jobs.NewTestJobStore(storageBackend, jobsMap)
		err := jobStore.SetJobCompleteStatus(jobID, jobs.Complete)
		assert.Nil(t, err)

		gotJob, err := jobStore.Get(jobID)
		assert.Nil(t, err)
		assert.Empty(t, gotJob.Output)
	})

	t.Run("error when job does not exist", func(t *testing.T) {
		storageBackend := &testStorageBackend{}
		jobStore := jobs.NewJobStore(storageBackend, tally.NewTestScope("test", map[string]string{}))
		jobID := "1234"
		expectedErrString := fmt.Sprintf("job: %s does not exist", jobID)

		err := jobStore.SetJobCompleteStatus(jobID, jobs.Complete)
		assert.EqualError(t, err, expectedErrString)

	})
}
