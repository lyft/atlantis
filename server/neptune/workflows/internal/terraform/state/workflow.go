package state

import (
	"net/url"
	"time"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/plugins"
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

type WorkflowStatus int
type WorkflowCompletionReason int

const (
	InProgressWorkflowStatus WorkflowStatus = iota
	CompleteWorkflowStatus
)

const (
	UnknownCompletionReason WorkflowCompletionReason = iota
	SuccessfulCompletionReason
	InternalServiceError
	TimeoutError
	SchedulingTimeoutError
	HeartbeatTimeoutError
	ActivityDurationTimeoutError
	SkippedCompletionReason
	ValidationFailedReason
)

type JobOutput struct {
	URL *url.URL

	// populated for plan jobs
	Summary terraform.PlanSummary
}

type JobAction struct {
	ID string

	// Info gives additional info about the action that can be used in things like tooltips
	Info string
}

func (a JobAction) ToGithubCheckRunAction() github.CheckRunAction {
	return github.CheckRunAction{
		Description: a.Info,
		Label:       a.ID,
	}
}

type JobActions struct {
	Actions []JobAction

	// Provides a form for messaging around the set actions
	Summary string
}

type Job struct {
	ID               string
	Output           *JobOutput
	OnWaitingActions JobActions
	Status           JobStatus
	StartTime        time.Time
	EndTime          time.Time
}

func (j *Job) toExternalJob() *plugins.JobState {
	return &plugins.JobState{
		ID: j.ID,

		// we can probably do this in a cleaner way
		Status:    plugins.JobStatus(string(j.Status)),
		StartTime: j.StartTime,
		EndTime:   j.EndTime,
	}
}

func (j Job) GetActions() JobActions {
	if j.Status == WaitingJobStatus {
		return j.OnWaitingActions
	}

	return JobActions{}
}

type WorkflowResult struct {
	Status WorkflowStatus
	Reason WorkflowCompletionReason
}

type Workflow struct {
	// Mode is used to determine which jobs are populated
	// Deploy mode: runs plan + apply jobs to create and deploy the changes merged into the default branch/force applied
	// PR mode: runs plan + validate jobs to create a terraform plan and run conftest checks against PR's diff
	Mode     *terraform.WorkflowMode
	Plan     *Job
	Validate *Job
	Apply    *Job
	Result   WorkflowResult
	ID       string
}

func (w *Workflow) ToExternalWorkflowState() *plugins.TerraformWorkflowState {
	return &plugins.TerraformWorkflowState{
		Plan:     getExternalJob(w.Plan),
		Apply:    getExternalJob(w.Apply),
		Validate: getExternalJob(w.Validate),
	}
}

func getExternalJob(j *Job) *plugins.JobState {
	if j == nil {
		return nil
	}
	return j.toExternalJob()
}
