package jobs

import (
	"io"

	"github.com/graymeta/stow"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/logging"
)

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_storage_backend.go StorageBackend

type StorageBackend interface {
	// Read logs from the storage backend. Must close the reader
	Read(key string) ([]string, error)

	// Write logs to the storage backend
	Write(key string, reader io.Reader) (success bool, err error)
}

type storageBackend struct {
	location      stow.Location
	logger        logging.SimpleLogging
	containerName string
}

func (s *storageBackend) Read(key string) ([]string, error) {
	return []string{}, nil
}

func (s *storageBackend) Write(key string, reader io.Reader) (success bool, err error) {
	s.logger.Info("Writing: %s to bucket: %s", key, s.containerName)
	err = stow.WalkContainers(s.location, stow.NoPrefix, 100, func(container stow.Container, err error) error {
		if err != nil {
			s.logger.Info(err.Error())
			return err
		}

		// Skip if not right container
		if container.Name() != s.containerName {
			return nil
		}

		_, err = container.Put(key, reader, 100, nil)
		if err != nil {
			s.logger.Info("error uploading to s3: ", err)
			return err
		}
		s.logger.Info("successfully uploaded logs for job: %s", key)

		return nil
	})

	if err != nil {
		return false, err
	}
	return true, nil
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

// Used when log persistence is not configured
type NoopStorageBackend struct{}

func (s *NoopStorageBackend) Read(key string) ([]string, error) {
	return []string{}, nil
}

func (s *NoopStorageBackend) Write(key string, reader io.Reader) (success bool, err error) {
	return false, nil
}
