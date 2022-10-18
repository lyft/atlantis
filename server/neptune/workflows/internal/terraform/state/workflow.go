package state

import (
	"net/url"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities/terraform"
)

type JobStatus string

const WorkflowStateChangeSignal = "terraform-workflow-state-change"

const (
	WaitingJobStatus    JobStatus = "waiting"
	InProgressJobStatus JobStatus = "in-progress"
	RejectedJobStatus   JobStatus = "rejected"
	FailedJobStatus     JobStatus = "failed"
	SuccessJobStatus    JobStatus = "success"
)

type JobOutput struct {
	URL *url.URL

	// populated for plan jobs
	Summary terraform.PlanSummary
}

type Job struct {
	Output *JobOutput
	Status JobStatus

	// Required fields
	Revision       string `json:"revision"`
	Repository     string `json:"repository"`
	Environment    string `json:"environment"`
	InitiatingUser string `json:"initiating_user"`
	StartTime      string `json:"start_time"`
	EndTime        string `json:"end_time"`
	ForceApply     bool   `json:"force_apply"`
	// Service name in the manifest.yaml
	Project string `json:"project"`
	// ProjectName in the atlantis.yaml
	RootName string `json:"root_name"`
}

type Workflow struct {
	Plan  *Job
	Apply *Job
}
