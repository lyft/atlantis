package websocket

import (
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/logging"
)

type partitionKeyGenerator interface {
	Generate(r *http.Request) (string, error)
}

type partitionRegistry interface {
	Register(key string, buffer chan string)
}

type Multiplexor interface {
	Handle(w http.ResponseWriter, r *http.Request) error
}

// Multiplexor is responsible for handling the data transfer between the storage layer
// and the registry. Note this is still a WIP as right now the registry is assumed to handle
// everything.
type multiplexor struct {
	writer       *Writer
	keyGenerator partitionKeyGenerator
	registry     partitionRegistry
}

func NewMultiplexor(log logging.Logger, keyGenerator partitionKeyGenerator, registry partitionRegistry) *multiplexor {
	upgrader := websocket.Upgrader{}
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	return &multiplexor{
		writer: &Writer{
			upgrader: upgrader,
			log:      log,
		},
		keyGenerator: keyGenerator,
		registry:     registry,
	}
}

// Handle should be called for a given websocket request. It blocks
// while writing to the websocket until the buffer is closed.
func (m *multiplexor) Handle(w http.ResponseWriter, r *http.Request) error {
	key, err := m.keyGenerator.Generate(r)

	if err != nil {
		return errors.Wrapf(err, "generating partition key")
	}

	// Buffer size set to 1000 to ensure messages get queued.
	// TODO: make buffer size configurable
	buffer := make(chan string, 1000)

	// spinning up a goroutine for this since we are attempting to block on the read side.
	go m.registry.Register(key, buffer)

	return errors.Wrapf(m.writer.Write(w, r, buffer), "writing to ws %s", key)
}
