package job

import (
	"context"
	"fmt"

	"github.com/runatlantis/atlantis/server/events/terraform/filter"
	"github.com/runatlantis/atlantis/server/jobs"
	"github.com/runatlantis/atlantis/server/neptune/logger"
)

type OutputLine struct {
	JobID string
	Line  string
}

type JobStore interface {
	Get(ctx context.Context, jobID string) (*Job, error)
	AppendOutput(jobID string, output string) error
	CloseAndPersistJob(ctx context.Context, jobID string, status JobStatus) error
	RemoveJob(jobID string)
}

type OutputHandler struct {
	// Main channel that receives output from the terraform client
	JobOutput chan *OutputLine

	// Storage  for plan/apply jobs
	JobStore JobStore

	// Registry to track active connections for a job
	ReceiverRegistry jobs.ReceiverRegistry
	LogFilter        filter.LogFilter
}

func (s *OutputHandler) Send(jobId string, msg string) {
	s.JobOutput <- &OutputLine{
		JobID: jobId,
		Line:  msg,
	}
}

func (s *OutputHandler) Handle(ctx context.Context) {
	for msg := range s.JobOutput {
		// Filter out log lines from job output
		if s.LogFilter.ShouldFilterLine(msg.Line) {
			continue
		}

		// Write logs to all active connections
		for ch := range s.ReceiverRegistry.GetReceivers(msg.JobID) {
			select {
			case ch <- msg.Line:
			default:
				s.ReceiverRegistry.RemoveReceiver(msg.JobID, ch)
			}
		}

		// Append new log to the output buffer for the job
		err := s.JobStore.AppendOutput(msg.JobID, msg.Line)
		if err != nil {
			logger.Warn(ctx, fmt.Sprintf("appending log: %s for job: %s: %v", msg.Line, msg.JobID, err))
		}
	}
}

func (s *OutputHandler) Register(ctx context.Context, jobID string, receiver chan string) {
	job, err := s.JobStore.Get(ctx, jobID)
	if err != nil || job == nil {
		logger.Error(ctx, fmt.Sprintf("getting job: %s, err: %v", jobID, err))
		return
	}

	// Back fill contents from the output buffer
	for _, line := range job.Output {
		receiver <- line
	}

	// Close connection if job is complete
	if job.Status == Complete {
		close(receiver)
		return
	}

	// add receiver to registry after backfilling contents from the buffer
	s.ReceiverRegistry.AddReceiver(jobID, receiver)

}

func (s *OutputHandler) CloseJob(ctx context.Context, jobID string) {
	// Close active connections and remove receivers from registry
	s.ReceiverRegistry.CloseAndRemoveReceiversForJob(jobID)

	// Update job status and persist to storage if configured
	if err := s.JobStore.CloseAndPersistJob(ctx, jobID, Complete); err != nil {
		logger.Error(ctx, fmt.Sprintf("updating jobs status to complete, %v", err))
	}
}
