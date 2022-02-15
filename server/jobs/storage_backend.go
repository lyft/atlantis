package jobs

import (
	"fmt"
	"io"
	"strings"

	"github.com/graymeta/stow"
	_ "github.com/graymeta/stow/s3"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/logging"
)

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_storage_backend.go StorageBackend

type StorageBackend interface {
	// Read logs from the storage backend. Must close the reader
	Read(key string) ([]string, error)

	// Write logs to the storage backend
	Write(key string, logs []string) (success bool, err error)
}

type storageBackend struct {
	location      stow.Location
	logger        logging.SimpleLogging
	containerName string
}

func (s *storageBackend) Read(key string) ([]string, error) {
	logs := []string{}
	err := stow.WalkContainers(s.location, stow.NoPrefix, 100, func(container stow.Container, err error) error {
		if err != nil {
			return err
		}

		// Found the right container
		if container.Name() == s.containerName {
			err := stow.Walk(container, stow.NoPrefix, 100, func(item stow.Item, err error) error {
				if err != nil {
					return err
				}

				// Found the right object
				if item.Name() == key {

					r, err := item.Open()
					if err != nil {
						return err
					}

					buf := new(strings.Builder)
					_, err = io.Copy(buf, r)
					if err != nil {
						return err
					}

					logs = strings.Split(buf.String(), "\n")
					return nil
				}
				return nil
			})
			if err != nil {
				return err
			}
			return nil
		}
		return nil
	})

	if err != nil {
		return []string{}, err
	}
	return logs, nil
}

func (s *storageBackend) Write(key string, logs []string) (success bool, err error) {
	err = stow.WalkContainers(s.location, stow.NoPrefix, 100, func(container stow.Container, err error) error {
		if err != nil {
			return err
		}

		// Found the right container
		if container.Name() == s.containerName {
			logsStr := strings.Join(logs, "\n")
			r := strings.NewReader(logsStr)
			size := int64(len(logsStr))

			item, err := container.Put(key, r, size, nil)
			if err != nil {
				return err
			}
			fmt.Println("Successfully uplodaded: %s", item.Name())
		}
		return nil
	})

	if err != nil {
		return false, err
	}
	return true, nil
}

func NewStorageBackend(jobs valid.Jobs, logger logging.SimpleLogging) (StorageBackend, error) {
	if jobs.StorageBackend.S3 != nil {
		config := stow.ConfigMap{}

		// Dial to s3
		location, err := stow.Dial("s3", config)
		if err != nil {
			return nil, err
		}

		return &storageBackend{
			location:      location,
			logger:        logger,
			containerName: jobs.StorageBackend.S3.BucketName,
		}, nil
	}
	return &NoopStorageBackend{}, nil
}

// Used when log persistence is not configured
type NoopStorageBackend struct{}

func (s *NoopStorageBackend) Read(key string) ([]string, error) {
	return []string{}, nil
}

func (s *NoopStorageBackend) Write(key string, logs []string) (success bool, err error) {
	return false, nil
}
