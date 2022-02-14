package websocket

import (
	"net/http"

	"github.com/uber-go/tally"
)

type InstrumentedMultiplexor struct {
	Multiplexor

	numWsConnections int64
	NumWsConnection  tally.Gauge
}

func NewInstrumentedMultiplexor(multiplexor Multiplexor, statsScope tally.Scope) Multiplexor {
	return &InstrumentedMultiplexor{
		Multiplexor:     multiplexor,
		NumWsConnection: statsScope.SubScope("api").SubScope("jobs").SubScope("websocket").Gauge("connections"),
	}
}

func (i *InstrumentedMultiplexor) Handle(w http.ResponseWriter, r *http.Request) error {
	i.numWsConnections += 1
	i.NumWsConnection.Update(float64(i.numWsConnections))

	defer func() {
		i.numWsConnections -= 1
		i.NumWsConnection.Update(float64(i.numWsConnections))
	}()

	return i.Multiplexor.Handle(w, r)
}
