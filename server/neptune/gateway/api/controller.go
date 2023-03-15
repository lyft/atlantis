package api

import (
	"context"
	"net/http"
)

type Controller[Request any] struct {
	RequestConverter RequestConverter[Request]
	Handler          Handler[Request]
}

type Handler[Request any] interface {
	Handle(ctx context.Context, request Request) error
}

type RequestConverter[T any] interface {
	Convert(from *http.Request) (T, error)
}

func (c *Controller[Request]) Handle(w http.ResponseWriter, request *http.Request) {
	internalRequest, err := c.RequestConverter.Convert(request)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
	}

	err = c.Handler.Handle(request.Context(), internalRequest)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}
}
