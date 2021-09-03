package handlers

import (
	"net/http"

	"github.com/gorilla/websocket"
)

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_websocket_handler.go WebsocketHandler

type WebsocketHandler interface {
	Upgrade(w http.ResponseWriter, r *http.Request, responseHeader http.Header) (WebsocketConnectionWrapper, error)
}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_websocket_response_writer.go WebsocketResponseWriter

type WebsocketConnectionWrapper interface {
	ReadMessage() (messageType int, p []byte, err error)
	WriteMessage(messageType int, data []byte) error
	SetCloseHandler(h func(code int, text string) error)
}

type DefaultWebsocketHandler struct {
	handler websocket.Upgrader
}

func NewWebsocketHandler() WebsocketHandler {
	h := websocket.Upgrader{}
	h.CheckOrigin = func(r *http.Request) bool { return true }
	return &DefaultWebsocketHandler{
		handler: h,
	}
}

func (wh *DefaultWebsocketHandler) Upgrade(w http.ResponseWriter, r *http.Request, responseHeader http.Header) (WebsocketConnectionWrapper, error) {
	return wh.handler.Upgrade(w, r, responseHeader)
}
