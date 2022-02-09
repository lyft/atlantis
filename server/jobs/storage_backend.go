package jobs

import "io"

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_storage_backend.go StorageBackend

type StorageBackend interface {
	// Checks the backend storage for the specified key
	IsKeyExists(key string) bool

	// Read logs from the storage backend. Must close the reader
	Read(key string) io.ReadCloser

	// Write logs to the storage backend
	Write(key string, logs []string) (success bool, err error)
}

// Used when log persistence is not configured
type NoopStorageBackend struct{}

func (s *NoopStorageBackend) IsKeyExists(key string) bool {
	return false
}

func (s *NoopStorageBackend) Read(key string) io.ReadCloser {
	return nil
}

func (s *NoopStorageBackend) Write(key string, logs []string) (success bool, err error) {
	return false, nil
}
