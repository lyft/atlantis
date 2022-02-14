package jobs

import (
	"fmt"
	"io"
	"strings"
	"sync"
)

type JobStatus int

const (
	Processing JobStatus = iota
	Complete
)

type Job struct {
	Output []string
	Status JobStatus
}

func NewJob(reader io.ReadCloser) *Job {
	buf := new(strings.Builder)
	_, err := io.Copy(buf, reader)
	if err != nil {
		return &Job{}
	}

	return &Job{
		Output: strings.Split(buf.String(), "/n"),
		Status: Complete,
	}
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
	SetCompleteJobStatus(jobID string, status JobStatus) error

	// Removes a job from the store
	RemoveJob(jobID string)
}

func NewJobStore(storageBackend StorageBackend) JobStore {
	return &LayeredJobStore{
		jobs:           map[string]*Job{},
		storageBackend: storageBackend,
	}
}

// Setup the job store for testing
func NewTestJobStore(storageBackend StorageBackend, jobs map[string]*Job) JobStore {
	return &LayeredJobStore{
		jobs:           jobs,
		storageBackend: storageBackend,
	}
}

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

func (j *LayeredJobStore) SetCompleteJobStatus(jobID string, status JobStatus) error {
	j.lock.Lock()
	defer j.lock.Unlock()

	// Error out when job dne
	if j.jobs[jobID] == nil {
		return fmt.Errorf("jobID does not exist")
	}

	// Error when job is already set to complete
	if job := j.jobs[jobID]; job.Status == Complete {
		return fmt.Errorf("job is already complete")
	}

	job := j.jobs[jobID]
	job.Status = Complete

	// Persist to storage backend
	ok, err := j.storageBackend.Write(jobID, job.Output)
	if err != nil {
		return fmt.Errorf("error persisting job: %s", jobID)
	}

	// Clear output buffers if successfully persisted
	if ok {
		delete(j.jobs, jobID)
	}

	return nil
}
