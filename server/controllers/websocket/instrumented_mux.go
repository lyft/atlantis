package websocket

import (
	"net/http"
	"sync/atomic"

	"github.com/uber-go/tally"
)

type InstrumentedMultiplexor struct {
	Multiplexor

	numWsConnections int64
	NumWsConnections tally.Gauge
}

func NewInstrumentedMultiplexor(multiplexor Multiplexor, statsScope tally.Scope) Multiplexor {
	return &InstrumentedMultiplexor{
		Multiplexor:      multiplexor,
		NumWsConnections: statsScope.SubScope("getprojectjobs").SubScope("websocket").Gauge("connections"),
	}
}

func (i *InstrumentedMultiplexor) Handle(w http.ResponseWriter, r *http.Request) error {
	atomic.AddInt64(&i.numWsConnections, 1)
	i.NumWsConnections.Update(float64(i.numWsConnections))

	defer func() {
		atomic.AddInt64(&i.numWsConnections, -1)
		i.NumWsConnections.Update(float64(i.numWsConnections))
	}()

	return i.Multiplexor.Handle(w, r)
}
