package util

import (
	"context"
	"sync"
	"sync/atomic"
)

// atomicError is a type-safe atomic value for errors.
// We use a struct{ error } to ensure consistent use of a concrete type.
type atomicError struct{ v atomic.Value }

func (a *atomicError) Store(err error) {
	a.v.Store(struct{ error }{err})
}
func (a *atomicError) Load() error {
	err, _ := a.v.Load().(struct{ error })
	return err.error
}

// A Gate is used to control the number of concurrent operations.
type Gate struct {
	gate    chan struct{}
	done    <-chan struct{}
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
	err     atomicError
	errOnce sync.Once
}

// NewGate returns a Gate that allows at most size concurrent operations and
// a context to use that will canceled if the gate.Cancel() is called.
// If the provided context is canceled or Canceled() is called Enter() will
// return false.  Internally, a sync.WaitGroup is used so each successful call
// to Enter() must have a corresponding call to Exit().
//
// An example of usage is below:
//
//	gate, ctx := NewGate(context.Background(), 2)
//	for i := 0; i < 8; i++ {
//		if !gate.Enter() {
//			break
//		}
//		go func() {
//			defer gate.Exit()
//			if err := Foo(ctx); err != nil {
//				gate.Cancel(err)
//			}
//		}()
//	}
//	if err := gate.Wait(); err != nil {
//		// Handle error
//	}
//
func NewGate(parent context.Context, size int) (*Gate, context.Context) {
	if size <= 0 {
		panic("gate: non-positve size argument")
	}
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	g := &Gate{
		gate:   make(chan struct{}, size),
		done:   ctx.Done(),
		ctx:    ctx,
		cancel: cancel,
	}
	return g, ctx
}

func (g *Gate) contextCanceled() {
	g.errOnce.Do(func() {
		err := g.ctx.Err()
		if err == nil {
			err = context.Canceled
		}
		g.err.Store(err)
	})
}

// Cancel sets the internal error and immediately cancels any pending gate
// operations.
func (g *Gate) Cancel(err error) {
	g.errOnce.Do(func() {
		g.err.Store(err)
		g.cancel()
	})
}

func (g *Gate) Err() error { return g.err.Load() }

// Wait blocks until the internal sync.WaitGroup counter is zero and returns
// the first error that occurred while the gate was active, if any.
func (g *Gate) Wait() error {
	g.wg.Wait()
	select {
	case <-g.done:
		g.contextCanceled()
	default:
	}
	return g.err.Load()
}

// Enter blocks until we are free to enter the gate and returns if the gate
// can be entered.
func (g *Gate) Enter() bool {
	select {
	case g.gate <- struct{}{}:
		select {
		case <-g.done:
			// canceled
		default:
			g.wg.Add(1)
			return true
		}
	case <-g.done:
		// canceled
	}
	g.contextCanceled()
	return false
}

// Exit must be called after any successful call to Enter().
func (g *Gate) Exit() {
	select {
	case <-g.gate:
	case <-g.done:
		g.contextCanceled()
	default:
		panic("gate: exit called without corresponding enter")
	}
	g.wg.Done()
}
