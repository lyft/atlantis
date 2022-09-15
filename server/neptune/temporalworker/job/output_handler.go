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

type JobStore interface {
	Get(jobID string) (*Job, error)
	Write(jobID string, output string) error
	RemoveJob(jobID string)

	// Activity context available
	Close(ctx context.Context, jobID string, status JobStatus) error
}

func NewOuptutHandler(
	jobStore JobStore,
	receiverRegistry ReceiverRegistry,
	logFilters valid.TerraformLogFilters,
	logger logging.Logger) *JobOutputHandler {
	logFilter := filter.LogFilter{
		Regexes: logFilters.Regexes,
	}

	jobOutputChan := make(chan *OutputLine)
	return &JobOutputHandler{
		JobOutput:        jobOutputChan,
		JobStore:         jobStore,
		ReceiverRegistry: receiverRegistry,
		LogFilter:        logFilter,
		Logger:           logger,
	}
}

type OutputHandler interface {
	Handle()
	ReadOutput(jobID string, ch <-chan terraform.Line) error
	Close(ctx context.Context, jobID string)
}

type JobOutputHandler struct {
	// Main channel that receives output from the terraform client
	JobOutput chan *OutputLine

	// Storage  for plan/apply jobs
	JobStore JobStore

	// Registry to track active connections for a job
	ReceiverRegistry ReceiverRegistry
	LogFilter        filter.LogFilter

	// Setting struct level Logger since not all methods have access to activity context
	Logger logging.Logger
}

func (s *JobOutputHandler) ReadOutput(jobID string, ch <-chan terraform.Line) error {
	for line := range ch {
		fmt.Println(line)

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

func (s *JobOutputHandler) Handle() {
	for msg := range s.JobOutput {
		// Filter out log lines from job output
		if s.LogFilter.ShouldFilterLine(msg.Line) {
			continue
		}

		s.ReceiverRegistry.Broadcast(*msg)

		// Append new log to the output buffer for the job
		err := s.JobStore.Write(msg.JobID, msg.Line)
		if err != nil {
			s.Logger.Warn(fmt.Sprintf("appending log: %s for job: %s: %v", msg.Line, msg.JobID, err))
		}
	}
}

// Called from inside an activity so activity context is available
func (s *JobOutputHandler) Close(ctx context.Context, jobID string) {
	// Close active connections and remove receivers from registry

	s.ReceiverRegistry.CloseAndRemoveReceiversForJob(jobID)

	// Update job status and persist to storage if configured
	if err := s.JobStore.Close(ctx, jobID, Complete); err != nil {
		s.Logger.Error(fmt.Sprintf("updating jobs status to complete, %v", err))
	}
}
