package api

import (
	"context"
	"net/http"
	"github.com/runatlantis/atlantis/server/neptune/sync"
)

type scheduler interface {
	Schedule(ctx context.Context, f sync.Executor) error
}

type Controller[Request any] struct {
	Scheduler scheduler
	RequestConverter RequestConverter[Request]
	Handler Handler[Request]
}

var deployVarKeys = []string{
	"owner",
	"repo",
	"root",
}

type Handler[Request any] interface {
	Handle(ctx context.Context, request Request) error
}

type RequestConverter[T any] interface {
	Convert(from *http.Request) (T, error)
}

func (c *Controller[Request]) RunAsync(w http.ResponseWriter, request *http.Request) {
	internalRequest, err := c.RequestConverter.Convert(request)

	if err != nil {
		// do something
	}

	c.Scheduler.Schedule(request.Context(), func(ctx context.Context) error {
		return c.Handler.Handle(ctx, internalRequest)
	})
}