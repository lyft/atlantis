package websocket

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/logging"
)

// PartitionRegistry is the registry holding each partition
// and is responsible for registering/deregistering new buffers
type PartitionRegistry interface {
	Register(key string, buffer chan string)
	Deregister(key string, buffer chan string)
}

// Multiplexor is responsible for handling the data transfer between the storage layer
// and the registry. Note this is still a WIP as right now the registry is assumed to handle
// everything.
type Multiplexor struct {
	writer   *Writer
	registry PartitionRegistry
}

func NewMultiplexor(log logging.SimpleLogging, registry PartitionRegistry) *Multiplexor {
	upgrader := websocket.Upgrader{}
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	return &Multiplexor{
		writer: &Writer{
			upgrader: upgrader,
			log:      log,
		},
		registry: registry,
	}
}

// Handle should be called for a given websocket request. It blocks
// while writing to the websocket until the buffer is closed.
func (m *Multiplexor) Handle(w http.ResponseWriter, r *http.Request) error {
	jobID, ok := mux.Vars(r)["job-id"]
	if !ok {
		return fmt.Errorf("internal error: no job ID in route")
	}

	// Buffer size set to 1000 to ensure messages get queued.
	// TODO: make buffer size configurable
	buffer := make(chan string, 1000)

	// spinning up a goroutine for this since we are attempting to block on the read side.
	go m.registry.Register(jobID, buffer)
	defer m.registry.Deregister(jobID, buffer)

	return errors.Wrapf(m.writer.Write(w, r, buffer), "writing to ws %s", jobID)
}
