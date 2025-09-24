package markdown

import (
	"bytes"
	_ "embed" //embedding files
	"fmt"
	"html/template"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
)

//go:embed templates/planconfirm.tmpl
var planConfirmStr string

//go:embed templates/checkrun.tmpl
var checkrunTemplateStr string

// panics if we can't read the template
var checkrunTemplate = template.Must(template.New("").Parse(checkrunTemplateStr))
var planConfirmTemplate = template.Must(template.New("").Parse(planConfirmStr))

type planconfirmTemplateData struct {
	Revision              string
	RevisionURL           string
	Pull                  int
	PullURL               string
	User                  string
	OnDefaultBranch       bool
	LatestOnDefaultBranch bool
}

type checkrunTemplateData struct {
	ApplyActionsSummary     string
	PlanStatus              string
	PlanLogURL              string
	ValidateStatus          string
	ValidateLogURL          string
	ApplyStatus             string
	ApplyLogURL             string
	InternalError           bool
	TimedOut                bool
	ActivityDurationTimeout bool
	SchedulingTimeout       bool
	HeartbeatTimeout        bool
	PRMode                  bool
	Skipped                 bool
	ValidationError         bool
	BypassedError           bool
	PlanSummary             string
	ValidateSummary         string
}

func RenderWorkflowStateTmpl(workflowState *state.Workflow) string {
	planStatus, planLogURL := getJobStatusAndOutput(workflowState.Plan)
	validateStatus, validateLogURL := getJobStatusAndOutput(workflowState.Validate)
	applyStatus, applyLogURL := getJobStatusAndOutput(workflowState.Apply)

	// we can probably pass in the completion reason but i like doing all the boolean
	// checking here if we can instead of in the template.
	internalError := workflowState.Result.Reason == state.InternalServiceError
	timedOut := workflowState.Result.Reason == state.TimeoutError
	activityDurationTimeout := workflowState.Result.Reason == state.ActivityDurationTimeoutError
	schedulingTimeout := workflowState.Result.Reason == state.SchedulingTimeoutError
	hearbeatTimeout := workflowState.Result.Reason == state.HeartbeatTimeoutError
	skipped := workflowState.Result.Reason == state.SkippedCompletionReason
	validation := workflowState.Result.Reason == state.ValidationFailedReason
	bypassed := workflowState.Result.Reason == state.BypassedFailedValidationReason
	var prMode bool
	if workflowState.Mode != nil {
		prMode = *workflowState.Mode == terraform.PR
	}

	var applyActionsSummary string
	if workflowState.Apply != nil {
		applyActionsSummary = workflowState.Apply.GetActions().Summary
	}

	var planSummary string
	var validateSummary string
	if prMode {
		if workflowState.Plan != nil && workflowState.Plan.IsComplete() && workflowState.Plan.Output != nil {
			planSummary = workflowState.Plan.Output.PlanSummary.String()
		}

		if workflowState.Validate != nil && workflowState.Validate.IsComplete() && workflowState.Validate.Output != nil {
			validateSummary = workflowState.Validate.Output.ValidateSummary.String()
		}
	}

	return renderTemplate(checkrunTemplate, checkrunTemplateData{
		PlanStatus:              planStatus,
		PlanLogURL:              planLogURL,
		PlanSummary:             planSummary,
		ValidateStatus:          validateStatus,
		ValidateLogURL:          validateLogURL,
		ValidateSummary:         validateSummary,
		ApplyStatus:             applyStatus,
		ApplyLogURL:             applyLogURL,
		PRMode:                  prMode,
		InternalError:           internalError,
		ValidationError:         validation,
		BypassedError:           bypassed,
		TimedOut:                timedOut,
		ActivityDurationTimeout: activityDurationTimeout,
		SchedulingTimeout:       schedulingTimeout,
		HeartbeatTimeout:        hearbeatTimeout,
		ApplyActionsSummary:     applyActionsSummary,
		Skipped:                 skipped,
	})
}

func RenderPlanConfirm(user string, commit github.Commit, deployedBranch string, deployedRevision string, repo github.Repo) string {
	data := planconfirmTemplateData{
		Revision:              deployedRevision,
		RevisionURL:           github.BuildRevisionURLMarkdown(repo.GetFullName(), deployedRevision),
		User:                  user,
		OnDefaultBranch:       commit.Branch == repo.DefaultBranch,
		LatestOnDefaultBranch: deployedBranch == repo.DefaultBranch,
	}

	return renderTemplate(planConfirmTemplate, data)
}

func getJobStatusAndOutput(jobState *state.Job) (string, string) {
	var status string
	var output string

	if jobState == nil {
		return status, output
	}

	return string(jobState.Status), jobState.Output.URL.String()
}

func renderTemplate(tmpl *template.Template, data interface{}) string {
	buf := &bytes.Buffer{}
	if err := tmpl.Execute(buf, data); err != nil {
		return fmt.Sprintf("Failed to render template, this is a bug: %v. Dumping the current data object as is: %s", err, data)
	}
	return buf.String()
}
