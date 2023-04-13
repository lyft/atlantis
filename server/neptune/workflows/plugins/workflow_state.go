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

type TerraformWorkflowStatus int

const (
	InProgressWorkflowStatus TerraformWorkflowStatus = iota
	CompleteWorkflowStatus
)

type Job struct {
	ID        string
	Status    JobStatus
	StartTime time.Time
	EndTime   time.Time
}

type TerraformWorkflowState struct {
	Plan     *Job
	Validate *Job
	Apply    *Job
}
