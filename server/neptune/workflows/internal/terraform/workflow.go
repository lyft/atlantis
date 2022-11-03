package terraform

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/config/logger"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/sideeffect"
	runner "github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/job"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type githubActivities interface {
	FetchRoot(ctx context.Context, request activities.FetchRootRequest) (activities.FetchRootResponse, error)
}

type terraformActivities interface {
	Cleanup(ctx context.Context, request activities.CleanupRequest) (activities.CleanupResponse, error)
	GetWorkerInfo(ctx context.Context) (*activities.GetWorkerInfoResponse, error)
}

// jobRunner runs a deploy plan/apply job
type jobRunner interface {
	Plan(ctx workflow.Context, localRoot *terraform.LocalRoot, jobID string) (activities.TerraformPlanResponse, error)
	Apply(ctx workflow.Context, localRoot *terraform.LocalRoot, jobID string, planFile string) error
}

type PlanStatus int
type PlanReviewSignalRequest struct {
	Status PlanStatus

	// TODO: Output this info to the checks UI
	User string
}

const (
	Approved PlanStatus = iota
	Rejected
)

const (
	PlanReviewSignalName   = "planreview"
	ScheduleToCloseTimeout = 30 * time.Minute
	HeartBeatTimeout       = 1 * time.Minute
)

func Workflow(ctx workflow.Context, request Request) error {
	runner := newRunner(ctx, request)

	// blocking call
	return runner.Run(ctx)
}

type Runner struct {
	GithubActivities    githubActivities
	TerraformActivities terraformActivities
	JobRunner           jobRunner
	Request             Request
	Store               *state.WorkflowStore
	RootFetcher         *RootFetcher
}

func newRunner(ctx workflow.Context, request Request) *Runner {
	var ta *activities.Terraform
	var ga *activities.Github

	cmdStepRunner := runner.CmdStepRunner{
		Activity: ta,
		Ref:      request.Repo.Ref,
	}

	parent := workflow.GetInfo(ctx).ParentWorkflowExecution

	return &Runner{
		GithubActivities:    ga,
		TerraformActivities: ta,
		Request:             request,
		JobRunner: runner.NewRunner(
			&cmdStepRunner,
			&runner.EnvStepRunner{
				CmdStepRunner: cmdStepRunner,
			},
			ta,
		),
		RootFetcher: &RootFetcher{
			Request: request,
			Ga:      ga,
			Ta:      ta,
		},

		// We have critical things relying on this notification so this workflow provides guarantees around this. (ie. compliance auditing)  There should
		// be no situation where we are deploying while this is failing.
		Store: state.NewWorkflowStore(
			func(s *state.Workflow) error {
				return workflow.SignalExternalWorkflow(ctx, parent.ID, parent.RunID, state.WorkflowStateChangeSignal, s).Get(ctx, nil)
			},
		),
	}
}

func (r *Runner) Plan(ctx workflow.Context, root *terraform.LocalRoot, serverURL fmt.Stringer) (activities.TerraformPlanResponse, error) {
	var response activities.TerraformPlanResponse
	jobID, err := sideeffect.GenerateUUID(ctx)
	if err != nil {
		return response, errors.Wrap(err, "generating job id")
	}

	// fail if we error here since all successive calls to update this will fail otherwise
	if err := r.Store.InitPlanJob(jobID, serverURL); err != nil {
		return response, errors.Wrap(err, "initializing job")
	}

	if err := r.Store.UpdatePlanJobWithStatus(state.InProgressJobStatus); err != nil {
		return response, newUpdateJobError(err, "unable to update job with in-progress status")
	}

	response, err = r.JobRunner.Plan(ctx, root, jobID.String())

	if err != nil {
		if e := r.Store.UpdatePlanJobWithStatus(state.FailedJobStatus); e != nil {
			// not returning UpdateJobError here since we want to surface the job failure itself
			logger.Error(ctx, "unable to update job with failed status, job failed with error. ", "err", err)
		}
		return response, newTerraformClientError(err, "running job")
	}
	if err := r.Store.UpdatePlanJobWithStatus(state.SuccessJobStatus, state.UpdateOptions{
		PlanSummary: response.Summary,
	}); err != nil {
		return response, newUpdateJobError(err, "unable to update job with success status")
	}

	return response, nil
}

func (r *Runner) waitForPlanReview(ctx workflow.Context, root *terraform.LocalRoot) PlanStatus {
	// Wait for plan review signal
	var planReview PlanReviewSignalRequest

	if root.Root.Plan.Approval == terraform.AutoApproval {
		return Approved
	}

	// if this root is configured for manual approval we block until we receive a signal
	signalChan := workflow.GetSignalChannel(ctx, PlanReviewSignalName)
	_ = signalChan.Receive(ctx, &planReview)

	return planReview.Status
}

func (r *Runner) Apply(ctx workflow.Context, root *terraform.LocalRoot, serverURL fmt.Stringer, planFile string) error {
	jobID, err := sideeffect.GenerateUUID(ctx)

	if err != nil {
		return errors.Wrap(err, "generating job id")
	}

	// fail if we error here since all successive calls to update this will fail otherwise
	if err := r.Store.InitApplyJob(jobID, serverURL); err != nil {
		return errors.Wrap(err, "initializing job")
	}

	planStatus := r.waitForPlanReview(ctx, root)

	if planStatus == Rejected {
		if err := r.Store.UpdateApplyJobWithStatus(state.RejectedJobStatus); err != nil {
			logger.Error(ctx, "unable to update job with rejected status.", "err", err)
		}
		return newPlanRejectedError()
	}

	if err := r.Store.UpdateApplyJobWithStatus(state.InProgressJobStatus, state.UpdateOptions{
		StartTime: time.Now(),
	}); err != nil {
		return newUpdateJobError(err, "unable to update job with success status")
	}

	err = r.JobRunner.Apply(ctx, root, jobID.String(), planFile)
	if err != nil {
		if e := r.Store.UpdateApplyJobWithStatus(state.FailedJobStatus, state.UpdateOptions{
			EndTime: time.Now(),
		}); e != nil {
			// not returning UpdateJobError here since we want to surface the job failure itself
			logger.Error(ctx, "unable to update job with failed status, job failed with error. ", "err", err)
		}
		return newTerraformClientError(err, "running job")
	}

	if err := r.Store.UpdateApplyJobWithStatus(state.SuccessJobStatus, state.UpdateOptions{
		EndTime: time.Now(),
	}); err != nil {
		return newUpdateJobError(err, "unable to update job with success status")
	}

	return nil
}

func (r *Runner) Run(ctx workflow.Context) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: ScheduleToCloseTimeout,
		HeartbeatTimeout:       HeartBeatTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			NonRetryableErrorTypes: []string{ClientErrorType},
		},
	})
	var response *activities.GetWorkerInfoResponse
	err := workflow.ExecuteActivity(ctx, r.TerraformActivities.GetWorkerInfo).Get(ctx, &response)
	if err != nil {
		return errors.Wrap(err, "getting worker info")
	}

	opts := workflow.GetActivityOptions(ctx)
	opts.TaskQueue = response.TaskQueue
	ctx = workflow.WithActivityOptions(ctx, opts)

	root, cleanup, err := r.RootFetcher.Fetch(ctx)
	if err != nil {
		return errors.Wrap(err, "fetching root")
	}
	defer func() {
		err := cleanup()

		if err != nil {
			logger.Warn(ctx, "error cleaning up local root", "err", err)
		}
	}()

	planResponse, err := r.Plan(ctx, root, response.ServerURL)
	if err != nil {
		return toExternalError(err, "running plan job")
	}

	if err := r.Apply(ctx, root, response.ServerURL, planResponse.PlanFile); err != nil {
		return toExternalError(err, "running apply job")
	}

	return nil
}

type ApplicationError struct {
	ErrType string
	Msg     string
}

func (e ApplicationError) Error() string {
	return fmt.Sprintf("Type: %s, Msg: %s", e.ErrType, e.Msg)
}

func (e ApplicationError) ToTemporalApplicationError() error {
	return temporal.NewApplicationError(
		e.Msg,
		e.ErrType,
		e,
	)
}

// toExternalError allows callers of the workflow to handle specific error types
// in whatever fashion.
// we use errors.As here to ensure that we're accounting for wrapped errors
func toExternalError(err error, msg string) error {
	var planRejected PlanRejectedError
	if errors.As(err, &planRejected) {
		e := ApplicationError{
			ErrType: planRejected.GetExternalType(),
			Msg:     errors.Wrap(err, msg).Error(),
		}

		return e.ToTemporalApplicationError()
	}

	var updateJobErr UpdateJobError
	if errors.As(err, &updateJobErr) {
		e := ApplicationError{
			ErrType: updateJobErr.GetExternalType(),
			// wrap original error to provide job type
			Msg: errors.Wrap(err, msg).Error(),
		}

		return e.ToTemporalApplicationError()
	}

	var terraformClientErr TerraformClientError
	if errors.As(err, &terraformClientErr) {
		e := ApplicationError{
			ErrType: terraformClientErr.GetExternalType(),
			// wrap original error to provide job type
			Msg: errors.Wrap(err, msg).Error(),
		}

		return e.ToTemporalApplicationError()
	}
	return errors.Wrap(err, msg)
}
