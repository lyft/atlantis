package job

import (
	"context"
	"fmt"
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
}

func NewStore(storageBackend StorageBackend) JobStore {
	return &StorageBackendJobStore{
		JobStore: &InMemoryStore{
			jobs: map[string]*Job{},
		},
		storageBackend: storageBackend,
	}
}

// Memory Job store deals with handling jobs in memory
type InMemoryStore struct {
	jobs map[string]*Job
	lock sync.RWMutex
}

func (m *InMemoryStore) Get(jobID string) (*Job, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	if m.jobs[jobID] == nil {
		return nil, nil
	}
	return m.jobs[jobID], nil
}

func (m *InMemoryStore) Write(jobID string, output string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Create new job if job dne
	if m.jobs[jobID] == nil {
		m.jobs[jobID] = &Job{}
	}

	if m.jobs[jobID].Status == Complete {
		return fmt.Errorf("cannot append to a complete job")
	}

	updatedOutput := append(m.jobs[jobID].Output, output)
	m.jobs[jobID].Output = updatedOutput
	return nil
}

func (m *InMemoryStore) Close(ctx context.Context, jobID string, status JobStatus) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Error out when job dne
	if m.jobs[jobID] == nil {
		return fmt.Errorf("job: %s does not exist", jobID)
	}

	// Error when job is already set to complete
	if job := m.jobs[jobID]; job.Status == Complete {
		return fmt.Errorf("job: %s is already complete", jobID)
	}

	job := m.jobs[jobID]
	job.Status = Complete
	return nil
}

func (m *InMemoryStore) RemoveJob(jobID string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	delete(m.jobs, jobID)
}

// Storage backend job store deals with handling jobs in backend storage
type StorageBackendJobStore struct {
	JobStore
	storageBackend StorageBackend
}

func (s *StorageBackendJobStore) Get(jobID string) (*Job, error) {
	// Get job from memory
	if jobInMem, _ := s.JobStore.Get(jobID); jobInMem != nil {
		return jobInMem, nil
	}

	// Get from storage backend if not in memory
	logs, err := s.storageBackend.Read(jobID)
	if err != nil {
		return nil, errors.Wrap(err, "reading from backend storage")
	}

	return &Job{
		Output: logs,
		Status: Complete,
	}, nil
}

func (s StorageBackendJobStore) Write(jobID string, output string) error {
	return s.JobStore.Write(jobID, output)
}

func (s *StorageBackendJobStore) Close(ctx context.Context, jobID string, status JobStatus) error {
	if err := s.JobStore.Close(ctx, jobID, status); err != nil {
		return err
	}

	job, err := s.JobStore.Get(jobID)
	if err != nil || job == nil {
		return errors.Wrapf(err, "retrieving job: %s from memory store", jobID)
	}
	// subScope := s.scope.SubScope("set_job_complete_status")
	// subScope.Counter("write_attempt").Inc(1)
	ok, err := s.storageBackend.Write(ctx, jobID, job.Output)
	if err != nil {
		return errors.Wrapf(err, "persisting job: %s", jobID)
	}

	// Remove from memory if successfully persisted
	if ok {
		s.JobStore.RemoveJob(jobID)
	}
	return nil
}

func (s *StorageBackendJobStore) RemoveJob(jobID string) {
	s.JobStore.RemoveJob(jobID)
}
