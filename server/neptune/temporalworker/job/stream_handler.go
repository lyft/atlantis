package job

import (
	"context"
	"fmt"

	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/terraform/filter"
	"github.com/runatlantis/atlantis/server/logging"
)

type OutputLine struct {
	JobID string
	Line  string
}

type StreamHandler interface {
	Stream(jobID string, msg string)
	Handle()
	CloseJob(ctx context.Context, jobID string) error
	CleanUp(ctx context.Context) error
}

func NewStreamHandler(
	jobStore Store,
	receiverRegistry ReceiverRegistry,
	logFilters valid.TerraformLogFilters,
	streamChan chan *OutputLine,
	logger logging.Logger,
) StreamHandler {

	logFilter := filter.LogFilter{
		Regexes: logFilters.Regexes,
	}

	return &streamHandler{
		JobOutput:        streamChan,
		Store:            jobStore,
		ReceiverRegistry: receiverRegistry,
		LogFilter:        logFilter,
		Logger:           logger,
	}
}

type streamHandler struct {
	JobOutput        chan *OutputLine
	Store            Store
	ReceiverRegistry ReceiverRegistry
	LogFilter        filter.LogFilter
	Logger           logging.Logger
}

func (s *streamHandler) Stream(jobID string, msg string) {
	s.JobOutput <- &OutputLine{
		JobID: jobID,
		Line:  msg,
	}
}

func (s *streamHandler) Handle() {
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

// Activity context since it's called from within an activity
func (s *streamHandler) CloseJob(ctx context.Context, jobID string) error {
	s.ReceiverRegistry.Close(ctx, jobID)
	return s.Store.Close(ctx, jobID, Complete)
}

func (s *streamHandler) CleanUp(ctx context.Context) error {
	s.ReceiverRegistry.CleanUp()
	return s.Store.Cleanup(ctx)
}
