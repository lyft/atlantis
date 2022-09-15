package job

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/terraform/filter"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/neptune/terraform"
)

type OutputLine struct {
	JobID string
	Line  string
}

type outputHandler interface {
	Handle()
	Read(jobID string, ch <-chan terraform.Line) error
	Close(ctx context.Context, jobID string)
}

func NewOuptutHandler(
	jobStore store,
	receiverRegistry receiverRegistry,
	logFilters valid.TerraformLogFilters,
	logger logging.Logger,
) *OutputHandler {

	logFilter := filter.LogFilter{
		Regexes: logFilters.Regexes,
	}

	jobOutputChan := make(chan *OutputLine)
	return &OutputHandler{
		JobOutput:        jobOutputChan,
		Store:            jobStore,
		ReceiverRegistry: receiverRegistry,
		LogFilter:        logFilter,
		Logger:           logger,
	}
}

type OutputHandler struct {
	JobOutput        chan *OutputLine
	Store            store
	ReceiverRegistry receiverRegistry
	LogFilter        filter.LogFilter
	Logger           logging.Logger
}

func (s *OutputHandler) Read(ctx context.Context, jobID string, ch <-chan terraform.Line) error {
	for line := range ch {
		if line.Err != nil {
			return errors.Wrap(line.Err, "executing command")
		}
		s.JobOutput <- &OutputLine{
			JobID: jobID,
			Line:  line.Line,
		}
	}
	return nil
}

func (s *OutputHandler) Handle() {
	for msg := range s.JobOutput {
		// Filter out log lines from job output
		if s.LogFilter.ShouldFilterLine(msg.Line) {
			continue
		}

		s.ReceiverRegistry.Broadcast(*msg)

		// Append new log to the output buffer for the job
		err := s.Store.Write(msg.JobID, msg.Line)
		if err != nil {
			s.Logger.Warn(fmt.Sprintf("appending log: %s for job: %s: %v", msg.Line, msg.JobID, err))
		}
	}
}

func (s *OutputHandler) Close(ctx context.Context, jobID string) {
	s.ReceiverRegistry.Close(ctx, jobID)

	// Update job status and persist to storage if configured
	if err := s.Store.Close(ctx, jobID, Complete); err != nil {
		s.Logger.Error(fmt.Sprintf("updating jobs status to complete, %v", err))
	}
}
