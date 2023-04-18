package plugins

import (
	"time"
)

type JobStatus string

const (
	WaitingJobStatus    JobStatus = "waiting"
	InProgressJobStatus JobStatus = "in-progress"
	RejectedJobStatus   JobStatus = "rejected"
	FailedJobStatus     JobStatus = "failed"
	SuccessJobStatus    JobStatus = "success"
)

// JobState represents the state of a job at a given time.
type JobState struct {
	ID        string
	Status    JobStatus
	StartTime time.Time
	EndTime   time.Time
}

// TerraformWorkflowState contains the state of all jobs in the workflow
// at a given time.  Note: jobs must always be checked for nil as they might
// not exist at certain points.
type TerraformWorkflowState struct {
	Plan     *JobState
	Validate *JobState
	Apply    *JobState
}
