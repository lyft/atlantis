package websocket

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/logging"
)

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_response_writer.go ResponseWriter

type ResponseWriter interface {
	http.ResponseWriter
}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_read_closer.go ReadCloser

type ReadCloser interface {
	io.ReadCloser
}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_partition_key_generator.go PartitionKeyGenerator

// PartitionKeyGenerator generates partition keys for the multiplexor
type PartitionKeyGenerator interface {
	Generate(r *http.Request) (string, error)
}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_partition_registry.go PartitionRegistry

// PartitionRegistry is the registry holding each partition
// and is responsible for registering/deregistering new buffers
type PartitionRegistry interface {
	Register(key string, buffer chan string)
	Deregister(key string, buffer chan string)
	IsKeyExists(key string) bool
}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_reader.go Reader

// Reader for storage backend.
type Reader interface {
	IsKeyExists(key string) bool
	Read(key string) io.ReadCloser
}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_writer.go Writer

// Writer for web socket connection.
type Writer interface {
	WriteFromChan(rw http.ResponseWriter, r *http.Request, input chan string) error
	WriteFromReader(rw http.ResponseWriter, r *http.Request, reader io.ReadCloser) error
}

// Multiplexor is responsible for handling the data transfer between the storage layer
// and the registry. Note this is still a WIP as right now the registry is assumed to handle
// everything.
type Multiplexor struct {
	writer               Writer
	keyGenerator         PartitionKeyGenerator
	registry             PartitionRegistry
	storageBackendReader Reader
}

func NewMultiplexor(log logging.SimpleLogging, keyGenerator PartitionKeyGenerator, registry PartitionRegistry, storageBackendReader Reader, writer Writer) *Multiplexor {
	upgrader := websocket.Upgrader{}
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	return &Multiplexor{
		writer:               writer,
		keyGenerator:         keyGenerator,
		registry:             registry,
		storageBackendReader: storageBackendReader,
	}
}

// Handle should be called for a given websocket request. It blocks
// while writing to the websocket until the buffer is closed.
func (m *Multiplexor) Handle(w http.ResponseWriter, r *http.Request) error {
	key, err := m.keyGenerator.Generate(r)

	if err != nil {
		return errors.Wrapf(err, "generating partition key")
	}

	// Serve from s3 if key exists.
	if m.storageBackendReader.IsKeyExists(key) {
		objReader := m.storageBackendReader.Read(key)
		return errors.Wrapf(m.writer.WriteFromReader(w, r, objReader), "writing to ws %s", key)
	}

	// check if the job ID exists before registering receiver
	if !m.registry.IsKeyExists(key) {
		return fmt.Errorf("invalid key: %s", key)
	}

	// Buffer size set to 1000 to ensure messages get queued.
	// TODO: make buffer size configurable
	buffer := make(chan string, 1000)

	// spinning up a goroutine for this since we are attempting to block on the read side.
	go m.registry.Register(key, buffer)
	defer m.registry.Deregister(key, buffer)

	return errors.Wrapf(m.writer.WriteFromChan(w, r, buffer), "writing to ws %s", key)
}
