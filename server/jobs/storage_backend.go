package jobs

import (
	"fmt"
	"io"

	"github.com/graymeta/stow"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/logging"
)

const PageSize = 100

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_storage_backend.go StorageBackend

type StorageBackend interface {
	// Read logs from the storage backend. Must close the reader
	Read(key string) ([]string, error)

	// Write logs to the storage backend
	Write(key string, reader io.Reader) error
}

type storageBackend struct {
	location      stow.Location
	logger        logging.SimpleLogging
	containerName string
}

func (s *storageBackend) Read(key string) ([]string, error) {
	return []string{}, nil
}

func (s *storageBackend) Write(key string, reader io.Reader) error {
	containerFound := false

	// Function to write to container
	writeFn := func(container stow.Container, err error) error {
		if err != nil {
			return errors.Wrapf(err, "walking containers at location: %s", s.location)
		}

		// Skip if not right container
		if container.Name() != s.containerName {
			return nil
		}

		containerFound = true
		_, err = container.Put(key, reader, 100, nil)
		if err != nil {
			s.logger.Warn(fmt.Sprintf("error uploading to %s", s.location), err)
			return err
		}
		s.logger.Info("successfully uploaded logs for job: %s", key)

		return nil
	}

	s.logger.Info("Writing: %s to bucket: %s", key, s.containerName)
	err := stow.WalkContainers(s.location, s.containerName, PageSize, writeFn)

	if !containerFound {
		return fmt.Errorf("container: %s not found at location: %s", s.containerName, s.location)
	}

	return err
}

func NewStorageBackend(jobs valid.Jobs, logger logging.SimpleLogging) (StorageBackend, error) {

	if jobs.StorageBackend == nil {
		return &NoopStorageBackend{}, nil
	}

	config := jobs.StorageBackend.GetConfigMap()
	backend := jobs.StorageBackend.GetConfiguredBackend()
	containerName := jobs.StorageBackend.GetContainerName()

	location, err := stow.Dial(backend, config)
	if err != nil {
		return nil, err
	}

	return &storageBackend{
		location:      location,
		logger:        logger,
		containerName: containerName,
	}, nil
}

type StorageBackendNotConfigured struct{}

func (s *StorageBackendNotConfigured) Error() string {
	return "storage backend is not configured"
}

// Used when log persistence is not configured
type NoopStorageBackend struct{}

func (s *NoopStorageBackend) Read(key string) ([]string, error) {
	return []string{}, nil
}

func (s *NoopStorageBackend) Write(key string, reader io.Reader) error {
	return &StorageBackendNotConfigured{}
}
