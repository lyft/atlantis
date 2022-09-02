package job

import (
	"fmt"

	"github.com/runatlantis/atlantis/server/events/terraform/filter"
	"github.com/runatlantis/atlantis/server/jobs"
	"github.com/runatlantis/atlantis/server/logging"
)

type OutputLine struct {
	JobID string
	Line  string
}

type ouptutHandler interface {
	// Send will enqueue the msg and wait for Handle() to receive the message.
	Send(jobID string, msg string)

	// Listens for msg from channel
	Handle()

	// Register registers a channel and blocks until it is caught up. Callers should call this asynchronously when attempting
	// to read the channel in the same goroutine
	Register(jobID string, receiver chan string)

	// Persists job to storage backend and marks operation complete
	CloseJob(jobID string)
}

// Wraps the original project command output handler with neptune specific logic
type OutputHandler struct {
	// Main channel that receives output from the terraform client
	JobOutput chan *OutputLine

	// Storage for plan/apply jobs
	JobStore jobs.JobStore

	// Registry to track active connections for a job
	ReceiverRegistry jobs.ReceiverRegistry

	Logger    logging.Logger
	LogFilter filter.LogFilter
}

func (s *OutputHandler) Send(jobId string, msg string) {
	s.JobOutput <- &OutputLine{
		JobID: jobId,
		Line:  msg,
	}
}

func (s *OutputHandler) Handle() {
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
			s.Logger.Warn(fmt.Sprintf("appending log: %s for job: %s: %v", msg.Line, msg.JobID, err))
		}
	}
}

func (s *OutputHandler) Register(jobID string, receiver chan string) {
	job, err := s.JobStore.Get(jobID)
	if err != nil || job == nil {
		s.Logger.Error(fmt.Sprintf("getting job: %s, err: %v", jobID, err))
		return
	}

	// Back fill contents from the output buffer
	for _, line := range job.Output {
		receiver <- line
	}

	// Close connection if job is complete
	if job.Status == jobs.Complete {
		close(receiver)
		return
	}

	// add receiver to registry after backfilling contents from the buffer
	s.ReceiverRegistry.AddReceiver(jobID, receiver)

}

func (s *OutputHandler) CloseJob(jobID string) {
	// Close active connections and remove receivers from registry
	s.ReceiverRegistry.CloseAndRemoveReceiversForJob(jobID)

	// Update job status and persist to storage if configured
	if err := s.JobStore.SetJobCompleteStatus(jobID, jobs.Complete); err != nil {
		s.Logger.Error(fmt.Sprintf("updating jobs status to complete, %v", err))
	}
}
