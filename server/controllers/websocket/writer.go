package websocket

import (
	"io"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/logging"
)

func NewWriter(log logging.SimpleLogging) *WebsocketWriter {
	upgrader := websocket.Upgrader{}
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	return &WebsocketWriter{
		upgrader: upgrader,
		log:      log,
	}
}

type WebsocketWriter struct {
	upgrader websocket.Upgrader

	//TODO: Remove dependency on atlantis logger here if we upstream this.
	log logging.SimpleLogging
}

func (w *WebsocketWriter) WriteFromChan(rw http.ResponseWriter, r *http.Request, input chan string) error {
	conn, err := w.upgrader.Upgrade(rw, r, nil)

	if err != nil {
		return errors.Wrap(err, "upgrading websocket connection")
	}

	// block on reading our input channel
	for msg := range input {
		if err := conn.WriteMessage(websocket.BinaryMessage, []byte("\r"+msg+"\n")); err != nil {
			w.log.Warn("Failed to write ws message: %s", err)
			return err
		}
	}

	// close ws conn after input channel is closed
	if err = conn.Close(); err != nil {
		w.log.Warn("Failed to close ws connection: %s", err)
	}
	return nil
}

func (w *WebsocketWriter) WriteFromReader(rw http.ResponseWriter, r *http.Request, reader io.ReadCloser) error {
	defer reader.Close()

	conn, err := w.upgrader.Upgrade(rw, r, nil)
	if err != nil {
		return errors.Wrap(err, "upgrading websocket connection")
	}

	// Read from the s3 client and write to the websocket.
	buf := make([]byte, 4)
	for {
		_, err := reader.Read(buf)
		if err == io.EOF {
			break
		}

		// Do not return if a write fails
		if err := conn.WriteMessage(websocket.BinaryMessage, buf); err != nil {
			w.log.Warn("Failed to write ws message: %s", err)
		}
	}

	// close ws conn after input channel is closed
	if err = conn.Close(); err != nil {
		w.log.Warn("Failed to close ws connection: %s", err)
	}
	return nil
}
