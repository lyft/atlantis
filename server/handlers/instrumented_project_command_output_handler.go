package handlers

import (
	"fmt"

	"github.com/uber-go/tally"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"
)

type InstrumentedProjectCommandOutputHandler struct {
	ProjectCommandOutputHandler
	numWSConnnections tally.Counter
	logger            logging.SimpleLogging
}

func NewInstrumentedProjectCommandOutputHandler(projectCmdOutput chan *models.ProjectCmdOutputLine,
	projectStatusUpdater ProjectStatusUpdater,
	projectJobURLGenerator ProjectJobURLGenerator,
	logger logging.SimpleLogging,
	scope tally.Scope) ProjectCommandOutputHandler {
	prjCmdOutputHandler := NewAsyncProjectCommandOutputHandler(
		projectCmdOutput,
		projectStatusUpdater,
		projectJobURLGenerator,
		logger,
	)
	return &InstrumentedProjectCommandOutputHandler{
		ProjectCommandOutputHandler: prjCmdOutputHandler,
		numWSConnnections:           scope.SubScope("getprojectjobs").SubScope("websocket").Gauge("connections"),
		logger:                      logger,
	}
}

func (p *InstrumentedProjectCommandOutputHandler) Register(projectInfo string, receiver chan string) {
	p.numWSConnnections.In(1)
	defer func() {
		// Log message to ensure numWSConnnections gauge is being updated properly.
		// [ORCA-955] TODO: Remove when removing the feature flag for log streaming.
		p.logger.Info(fmt.Sprintf("Decreasing num of ws connections for project: %s", projectInfo))
		p.numWSConnnections.Update(-1)
	}()
	p.ProjectCommandOutputHandler.Register(projectInfo, receiver)
}

func (p *InstrumentedProjectCommandOutputHandler) Deregister(projectInfo string, receiver chan string) {
	p.numWSConnnections.Inc()
	defer func() {
		// Log message to ensure numWSConnnections gauge is being updated properly.
		// [ORCA-955] TODO: Remove when removing the feature flag for log streaming.
		p.logger.Info(fmt.Sprintf("Decreasing num of ws connections for project: %s", projectInfo))
		p.numWSConnnections.Dec()
	}()
	p.ProjectCommandOutputHandler.Deregister(projectInfo, receiver)
}
