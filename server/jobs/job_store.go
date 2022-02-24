package jobs

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/pkg/errors"
)

type JobStatus int

const (
	Processing JobStatus = iota
	Complete
)

type Job struct {
	Output []string
	Status JobStatus

	readIndex  int
	readBuffer string
}

func (j *Job) GetReader() io.Reader {
	logs := strings.Join(j.Output, "\n")
	return strings.NewReader(logs)
}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_job_store.go JobStore

type JobStore interface {
	// Gets the job from the in memory buffer, if available and if not, reaches to the storage backend
	// Returns an empty job with error if not in storage backend
	Get(jobID string) (Job, error)

	// Appends a given string to a job's output if the job is not complete yet
	AppendOutput(jobID string, output string) error

	// Sets a job status to complete and triggers any associated workflow,
	// e.g: if the status is complete, the job is flushed to the associated storage backend
	SetJobCompleteStatus(jobID string, status JobStatus) error

	// Removes a job from the store
	RemoveJob(jobID string)
}

func NewJobStore(storageBackend StorageBackend) *LayeredJobStore {
	return &LayeredJobStore{
		jobs:           map[string]*Job{},
		storageBackend: storageBackend,
	}
}

// Setup job store for testing
func NewTestJobStore(storageBackend StorageBackend, jobs map[string]*Job) *LayeredJobStore {
	return &LayeredJobStore{
		jobs:           jobs,
		storageBackend: storageBackend,
	}
}

// layeredJobStore is a job store with one or more than one layers of persistence
// storageBackend in this case
type LayeredJobStore struct {
	jobs           map[string]*Job
	storageBackend StorageBackend
	lock           sync.RWMutex
}

func (j *LayeredJobStore) Get(jobID string) (Job, error) {
	// Get from memory if available
	if job, ok := j.GetJobFromMemory(jobID); ok {
		return job, nil
	}

	// Get from storage backend if not in memory.
	logs, err := j.storageBackend.Read(jobID)
	if err != nil {
		return Job{}, err
	}

	// If read from storage backend, mark job complete so that the conn
	// can be closed
	return Job{
		Output: logs,
		Status: Complete,
	}, nil
}

func (j *LayeredJobStore) GetJobFromMemory(jobID string) (Job, bool) {
	j.lock.RLock()
	defer j.lock.RUnlock()

	if j.jobs[jobID] == nil {
		return Job{}, false
	}
	return *j.jobs[jobID], true
}

func (j *LayeredJobStore) AppendOutput(jobID string, output string) error {
	j.lock.Lock()
	defer j.lock.Unlock()

	// Create new job if job dne
	if j.jobs[jobID] == nil {
		j.jobs[jobID] = &Job{}
	}

	if j.jobs[jobID].Status == Complete {
		return fmt.Errorf("cannot append to a complete job")
	}

	updatedOutput := append(j.jobs[jobID].Output, output)
	j.jobs[jobID].Output = updatedOutput
	return nil
}

func (j *LayeredJobStore) RemoveJob(jobID string) {
	j.lock.Lock()
	defer j.lock.Unlock()

	delete(j.jobs, jobID)
}

func (j *LayeredJobStore) SetJobCompleteStatus(jobID string, status JobStatus) error {
	j.lock.Lock()
	defer j.lock.Unlock()

	// Error out when job dne
	if j.jobs[jobID] == nil {
		return fmt.Errorf("job: %s does not exist", jobID)
	}

	// Error when job is already set to complete
	if job := j.jobs[jobID]; job.Status == Complete {
		return fmt.Errorf("job: %s is already complete", jobID)
	}

	job := j.jobs[jobID]
	job.Status = Complete

	// Persist to storage backend and return error if error other than StorageBackendNotConfigured
	err := j.storageBackend.Write(jobID, job.GetReader())
	if err == nil {
		// Only remove from memory if logs are persisted successfully
		delete(j.jobs, jobID)
	} else if !errors.Is(err, &StorageBackendNotConfigured{}) {
		return errors.Wrapf(err, "error persisting job: %s", jobID)
	}

	return nil
}
