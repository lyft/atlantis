package websocket

import (
	"net/http"

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
		NumWsConnection: statsScope.SubScope("api").SubScope("jobs").SubScope("websocket").Gauge("connections"),
		logger:          logger,
	}
}

func (i *InstrumentedMultiplexor) Handle(w http.ResponseWriter, r *http.Request) error {
	i.numWsConnections += 1
	i.NumWsConnection.Update(float64(i.numWsConnections))
	i.logger.Info("Opening new ws connection")

	defer func() {
		i.numWsConnections -= 1
		i.NumWsConnection.Update(float64(i.numWsConnections))
		i.logger.Info("Closing ws connection")
	}()

	return i.Multiplexor.Handle(w, r)
}
