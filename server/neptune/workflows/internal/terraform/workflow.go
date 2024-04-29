package terraform

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/conftest"

	key "github.com/runatlantis/atlantis/server/neptune/context"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/sideeffect"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/gate"
	runner "github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/job"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type githubActivities interface {
	GithubFetchRoot(ctx context.Context, request activities.FetchRootRequest) (activities.FetchRootResponse, error)
}

type terraformActivities interface {
	Cleanup(ctx context.Context, request activities.CleanupRequest) (activities.CleanupResponse, error)
	GetWorkerInfo(ctx context.Context) (*activities.GetWorkerInfoResponse, error)
}

// jobRunner runs a deploy plan/apply job
type jobRunner interface {
	Plan(ctx workflow.Context, localRoot *terraform.LocalRoot, jobID string, workflowMode terraform.WorkflowMode) (activities.TerraformPlanResponse, error)
	Apply(ctx workflow.Context, localRoot *terraform.LocalRoot, jobID string, planFile string) error
	Validate(ctx workflow.Context, localRoot *terraform.LocalRoot, jobID string, showFile string) ([]activities.ValidationResult, error)
}

const (
	// the length of our longest allowed activity. for now we've set this to 60 minutes
	// as that is the default terraform timeout, we can change as necessary.
	StartToCloseTimeout = 60 * time.Minute

	// the length of time we're willing to wait without a heartbeat from our activities.
	// if we timeout here its indicative that our worker process has died.  This in
	// combination with ScheduleToStart allows us to react to worker failures and schedule
	// the entire workflow on another worker process.
	// Note this value is tied to the heartbeat frequency specified in our activities.
	HeartBeatTimeout = 1 * time.Minute

	// the length of time we will wait to schedule activities
	// on a given task queue, if this times out our tq might be starved.
	// We pick 5 minutes since it's rare for a tf operation to last
	// this long and when it does, more often than not we see it last
	// for more than 10 minutes
	// we can tune this value as necessary.
	ScheduleToStartTimeout = 5 * time.Minute

	// 1 week timeout for review gate, this is an application level timeout which ensures
	// we are not running the workflow forever when waiting for confirmation/rejection
	// of a plan.
	ReviewGateTimeout = 24 * time.Hour * 7
)

func Workflow(ctx workflow.Context, request Request) (Response, error) {
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
	MetricsHandler      client.MetricsHandler
	ReviewGate          *gate.Review
}

func newRunner(ctx workflow.Context, request Request) *Runner {
	var ta *activities.Terraform
	var ga *activities.Github

	cmdStepRunner := runner.CmdStepRunner{
		Activity: ta,
	}

	parent := workflow.GetInfo(ctx).ParentWorkflowExecution

	metricsHandler := workflow.GetMetricsHandler(ctx)

	// We have critical things relying on this notification so this workflow provides guarantees around this. (ie. compliance auditing)  There should
	// be no situation where we are deploying while this is failing.
	store := state.NewWorkflowStore(
		func(s *state.Workflow) error {
			// in adhoc mode we have no parent workflow to signal
			if request.WorkflowMode == terraform.Adhoc {
				return nil
			}
			return workflow.SignalExternalWorkflow(ctx, parent.ID, parent.RunID, state.WorkflowStateChangeSignal, s).Get(ctx, nil)
		},
		request.WorkflowMode,
		request.DeploymentID,
	)

	return &Runner{
		ReviewGate: &gate.Review{
			MetricsHandler: metricsHandler,
			Timeout:        ReviewGateTimeout,
			Client:         store,
		},
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
		MetricsHandler: metricsHandler,
		Store:          store,
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

	if strings.HasSuffix(root.Path, "simple") {
		_ = workflow.Sleep(ctx, 20*time.Second)
	}

	response, err = r.JobRunner.Plan(ctx, root, jobID.String(), r.Request.WorkflowMode)

	if err != nil {
		if e := r.Store.UpdatePlanJobWithStatus(state.FailedJobStatus); e != nil {
			// not returning UpdateJobError here since we want to surface the job failure itself
			workflow.GetLogger(ctx).Error("unable to update job with failed status, job failed with error. ", key.ErrKey, err)
		}
		return response, errors.Wrap(err, "running job")
	}
	if err := r.Store.UpdatePlanJobWithStatus(state.SuccessJobStatus, state.UpdateOptions{
		PlanSummary: response.Summary,
	}); err != nil {
		return response, newUpdateJobError(err, "unable to update job with success status")
	}

	return response, nil
}

func (r *Runner) Validate(ctx workflow.Context, root *terraform.LocalRoot, serverURL *url.URL, showFile string) ([]activities.ValidationResult, error) {
	jobID, err := sideeffect.GenerateUUID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "generating job id")
	}

	// fail if we error here since all successive calls to update this will fail otherwise
	if err := r.Store.InitValidateJob(jobID, serverURL); err != nil {
		return nil, errors.Wrap(err, "initializing job")
	}

	if err := r.Store.UpdateValidateJobWithStatus(state.InProgressJobStatus, state.UpdateOptions{
		StartTime: time.Now(),
	}); err != nil {
		return nil, newUpdateJobError(err, "unable to update job with in-progress status")
	}

	validateResults, err := r.JobRunner.Validate(ctx, root, jobID.String(), showFile)
	if err != nil {
		if e := r.Store.UpdateValidateJobWithStatus(state.FailedJobStatus, state.UpdateOptions{
			EndTime: time.Now(),
		}); e != nil {
			// not returning UpdateJobError here since we want to surface the job failure itself
			workflow.GetLogger(ctx).Error("unable to update job with failed status, job failed with error. ", key.ErrKey, e)
		}
		return nil, errors.Wrap(err, "running job")
	}

	if containsFailure(validateResults) {
		if e := r.Store.UpdateValidateJobWithStatus(state.FailedJobStatus, state.UpdateOptions{
			EndTime:         time.Now(),
			ValidateSummary: conftest.NewValidateSummaryFromResults(validateResults),
		}); e != nil {
			// not returning UpdateJobError here since we want to surface the job failure itself
			workflow.GetLogger(ctx).Error("unable to update job with failed status, job failed with error. ", key.ErrKey, e)
		}
		// even if we fail, we still want to return the results for processing the failed policies
		return validateResults, nil
	}

	if err := r.Store.UpdateValidateJobWithStatus(state.SuccessJobStatus, state.UpdateOptions{
		EndTime:         time.Now(),
		ValidateSummary: conftest.NewValidateSummaryFromResults(validateResults),
	}); err != nil {
		return nil, newUpdateJobError(err, "unable to update job with success status")
	}

	return validateResults, nil
}

func containsFailure(results []activities.ValidationResult) bool {
	for _, result := range results {
		if result.Status == activities.Fail {
			return true
		}
	}
	return false
}

func (r *Runner) Apply(ctx workflow.Context, root *terraform.LocalRoot, serverURL fmt.Stringer, planResponse activities.TerraformPlanResponse) error {
	jobID, err := sideeffect.GenerateUUID(ctx)

	if err != nil {
		return errors.Wrap(err, "generating job id")
	}

	// fail if we error here since all successive calls to update this will fail otherwise
	if err := r.Store.InitApplyJob(jobID, serverURL); err != nil {
		return errors.Wrap(err, "initializing job")
	}

	planStatus, err := r.ReviewGate.Await(ctx, root.Root, planResponse.Summary)
	if err != nil {
		workflow.GetLogger(ctx).Error("error waiting for plan review.", key.ErrKey, err)
		return newPlanRejectedError()
	}

	if planStatus == gate.Rejected {
		if err := r.Store.UpdateApplyJobWithStatus(state.RejectedJobStatus); err != nil {
			workflow.GetLogger(ctx).Error("unable to update job with rejected status.", key.ErrKey, err)
		}
		return newPlanRejectedError()
	}

	if err := r.Store.UpdateApplyJobWithStatus(state.InProgressJobStatus, state.UpdateOptions{
		StartTime: time.Now(),
	}); err != nil {
		return newUpdateJobError(err, "unable to update job with success status")
	}

	err = r.JobRunner.Apply(ctx, root, jobID.String(), planResponse.PlanFile)
	if err != nil {
		if err := r.Store.UpdateApplyJobWithStatus(state.FailedJobStatus, state.UpdateOptions{
			EndTime: time.Now(),
		}); err != nil {
			// not returning UpdateJobError here since we want to surface the job failure itself
			workflow.GetLogger(ctx).Error("unable to update job with failed status, job failed with error. ", key.ErrKey, err)
		}
		return errors.Wrap(err, "running job")
	}

	if err := r.Store.UpdateApplyJobWithStatus(state.SuccessJobStatus, state.UpdateOptions{
		EndTime: time.Now(),
	}); err != nil {
		return newUpdateJobError(err, "unable to update job with success status")
	}

	return nil
}

func (r *Runner) Run(ctx workflow.Context) (Response, error) {
	var err error
	var resp Response
	// make sure we are updating state on completion.
	defer func() {
		reason := state.SuccessfulCompletionReason

		if r := recover(); r != nil || err != nil {
			reason = state.InternalServiceError
		}

		var appErr *temporal.ApplicationError
		if errors.As(err, &appErr) {
			if appErr.Type() == PlanRejectedErrorType {
				reason = state.SkippedCompletionReason
			}
		}

		// check for any timeouts that percolated up
		var timeoutErr *temporal.TimeoutError
		if errors.As(err, &timeoutErr) {
			switch timeoutErr.TimeoutType() {
			case enums.TIMEOUT_TYPE_HEARTBEAT:
				reason = state.HeartbeatTimeoutError
			case enums.TIMEOUT_TYPE_START_TO_CLOSE:
				reason = state.ActivityDurationTimeoutError
			case enums.TIMEOUT_TYPE_SCHEDULE_TO_START:
				reason = state.SchedulingTimeoutError
			default:
				reason = state.TimeoutError
			}
		}

		if containsFailure(resp.ValidationResults) {
			reason = state.ValidationFailedReason
		}

		updateErr := r.Store.UpdateCompletion(state.WorkflowResult{
			Status: state.CompleteWorkflowStatus,
			Reason: reason,
		})
		if updateErr != nil {
			workflow.GetLogger(ctx).Warn("error updating completion status", key.ErrKey, err)
		}
	}()
	resp, err = r.run(ctx)
	return resp, err
}

func (r *Runner) run(ctx workflow.Context) (Response, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: StartToCloseTimeout,
	})
	var response *activities.GetWorkerInfoResponse
	err := workflow.ExecuteActivity(ctx, r.TerraformActivities.GetWorkerInfo).Get(ctx, &response)
	if err != nil {
		return Response{}, r.toExternalError(err, "getting worker info")
	}

	ctx = workflow.WithTaskQueue(ctx, response.TaskQueue)
	ctx = workflow.WithHeartbeatTimeout(ctx, HeartBeatTimeout)
	ctx = workflow.WithScheduleToStartTimeout(ctx, ScheduleToStartTimeout)

	root, cleanup, err := r.RootFetcher.Fetch(ctx)
	if err != nil {
		return Response{}, r.toExternalError(err, "fetching root")
	}

	defer func() {
		r.executeCleanup(ctx, cleanup)
	}()

	planResponse, err := r.Plan(ctx, root, response.ServerURL)
	if err != nil {
		return Response{}, r.toExternalError(err, "running plan job")
	}

	if r.Request.WorkflowMode == terraform.Adhoc {
		return Response{}, nil
	}

	if r.Request.WorkflowMode == terraform.PR {
		validationResults, err := r.Validate(ctx, root, response.ServerURL, planResponse.PlanJSONFile)
		if err != nil {
			return Response{}, r.toExternalError(err, "running validate job")
		}
		resp := Response{
			ValidationResults: validationResults,
			WorkflowState:     r.Store.GetStateCopy(),
		}
		return resp, nil
	}

	if err = r.Apply(ctx, root, response.ServerURL, planResponse); err != nil {
		return Response{}, r.toExternalError(err, "running apply job")
	}
	return Response{}, nil
}

func (r *Runner) executeCleanup(ctx workflow.Context, handlers ...func(workflow.Context) error) {
	// create a new disconnected ctx since we want this run even in the event of
	// cancellation
	if temporal.IsCanceledError(ctx.Err()) {
		var cancel workflow.CancelFunc
		ctx, cancel = workflow.NewDisconnectedContext(ctx)
		defer cancel()
	}

	// cap these retries since this we don't want to block in the event we fail to do so.
	ctx = workflow.WithRetryPolicy(ctx, temporal.RetryPolicy{
		MaximumAttempts: 3,
	})

	for _, h := range handlers {
		cleanupErr := h(ctx)

		if cleanupErr != nil {
			workflow.GetLogger(ctx).Warn("error cleaning up local root", key.ErrKey, cleanupErr)
		}
	}
}

type ApplicationError struct {
	ErrType string
	Msg     string
}

func (e ApplicationError) Error() string {
	return fmt.Sprintf("Type: %s, Msg: %s", e.ErrType, e.Msg)
}

func (e ApplicationError) ToTemporalApplicationError() error {
	return temporal.NewNonRetryableApplicationError(
		e.Msg,
		e.ErrType,
		e,
	)
}

// toExternalError allows callers of the workflow to handle specific error types
// in whatever fashion.
// we use errors.As here to ensure that we're accounting for wrapped errors
func (r *Runner) toExternalError(err error, msg string) error {
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

	var timeoutErr *temporal.TimeoutError
	if errors.As(err, &timeoutErr) {
		switch timeoutErr.TimeoutType() {
		case enums.TIMEOUT_TYPE_SCHEDULE_TO_START:
			// this likely means our worker died, so return a retryable
			// error which would allow the workflow to get scheduled
			// on a new worker.
			return temporal.NewApplicationErrorWithCause(
				timeoutErr.Message(),
				SchedulingError,
				timeoutErr,
			)
		}
	}

	// let's default to non-retryable for now since we don't know if the error would happen
	// pre-apply or post-apply which determines basically whether we can retry this workflow as is.
	return temporal.NewNonRetryableApplicationError(msg, UnknownErrorType, err)
}
