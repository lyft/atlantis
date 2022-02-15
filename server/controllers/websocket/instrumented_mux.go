package websocket

import (
	"net/http"
	"sync/atomic"

	"github.com/runatlantis/atlantis/server/logging"
	"github.com/uber-go/tally"
)

type InstrumentedMultiplexor struct {
	Multiplexor

	numWsConnections int64
	NumWsConnection  tally.Gauge
	logger           logging.SimpleLogging
}

func NewInstrumentedMultiplexor(multiplexor Multiplexor, statsScope tally.Scope, logger logging.SimpleLogging) Multiplexor {
	return &InstrumentedMultiplexor{
		Multiplexor:     multiplexor,
		NumWsConnection: statsScope.SubScope("getprojectjobs").SubScope("websocket").Gauge("connections"),
		logger:          logger,
	}
}

func (i *InstrumentedMultiplexor) Handle(w http.ResponseWriter, r *http.Request) error {
	atomic.AddInt64(&i.numWsConnections, 1)
	i.NumWsConnection.Update(float64(i.numWsConnections))

	defer func() {
		atomic.AddInt64(&i.numWsConnections, -1)
		i.NumWsConnection.Update(float64(i.numWsConnections))
	}()

	return i.Multiplexor.Handle(w, r)
}
