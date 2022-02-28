package jobs

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/lyft/feature"
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

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_job_store.go JobStore

type JobStore interface {
	// Gets the job from the in memory buffer, if available and if not, reaches to the storage backend
	// Returns an empty job with error if not in storage backend
	Get(jobID string) (Job, error)

	// Appends a given string to a job's output if the job is not complete yet
	AppendOutput(jobID string, output string) error

	// Sets a job status to complete and triggers any associated workflow,
	// e.g: if the status is complete, the job is flushed to the associated storage backend
	SetJobCompleteStatus(jobID string, fullRepoName string, status JobStatus) error

	// Removes a job from the store
	RemoveJob(jobID string)
}

func NewJobStore(storageBackend StorageBackend, featureAllocator feature.Allocator) *LayeredJobStore {
	return &LayeredJobStore{
		jobs:             map[string]*Job{},
		storageBackend:   storageBackend,
		FeatureAllocator: featureAllocator,
	}
}

// Setup job store for testing
func NewTestJobStore(storageBackend StorageBackend, jobs map[string]*Job, featureAllocator feature.Allocator) *LayeredJobStore {
	return &LayeredJobStore{
		jobs:             jobs,
		storageBackend:   storageBackend,
		FeatureAllocator: featureAllocator,
	}
}

// layeredJobStore is a job store with one or more than one layers of persistence
// storageBackend in this case
type LayeredJobStore struct {
	jobs             map[string]*Job
	storageBackend   StorageBackend
	lock             sync.RWMutex
	FeatureAllocator feature.Allocator
	Logger           logging.SimpleLogging
}

func (j *LayeredJobStore) Get(jobID string) (Job, error) {
	// Get from memory if available
	if job, ok := j.GetJobFromMemory(jobID); ok {
		return job, nil
	}

	logs := []string{}

	// Using jobID as fullRepoName since we can't have repo specific feature allocator
	// since this method is called when a user visits a job URL which does not have any info about
	// the repository.
	shouldAllocate, err := j.FeatureAllocator.ShouldAllocate(feature.LogPersistence, jobID)
	if err != nil {
		j.Logger.Err("unable to allocate for feature: %s, error: %s", feature.LogPersistence, err)
	}

	// Get from storage backend if not in memory.
	if shouldAllocate {
		logs, err = j.storageBackend.Read(jobID)
		if err != nil {
			return Job{}, err
		}
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

func (j *LayeredJobStore) SetJobCompleteStatus(jobID string, fullRepoName string, status JobStatus) error {
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

	// Writing to backend storage supports repo based feature allocator since we have the repo full name
	shouldAllocate, err := j.FeatureAllocator.ShouldAllocate(feature.LogPersistence, fullRepoName)
	if err != nil {
		j.Logger.Err("unable to allocate for feature: %s, error: %s", feature.LogPersistence, err)
	}
	ok := false
	if shouldAllocate {
		ok, err = j.storageBackend.Write(jobID, job.Output)
		if err != nil {
			return errors.Wrapf(err, "error persisting job: %s", jobID)
		}
	}

	// Only remove from memory if logs are persisted successfully
	if ok {
		delete(j.jobs, jobID)
	}

	return nil
}
