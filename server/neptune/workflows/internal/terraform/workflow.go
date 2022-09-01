package terraform

import (
	"net/url"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/config/logger"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/job"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/root"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/sideeffect"
	job_runner "github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/job"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/job/step/cmd"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/job/step/env"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"go.temporal.io/sdk/workflow"
)

type workflowActivities struct {
	*activities.Terraform
	*activities.Github
}

// jobRunner runs a deploy plan/apply job
type jobRunner interface {
	Run(ctx workflow.Context, job job.Job, localRoot *root.LocalRoot) (string, error)
}

type PlanStatus int
type PlanReview struct {
	Status PlanStatus
}

const (
	Approved PlanStatus = iota
	Rejected
)

func Workflow(ctx workflow.Context, request Request) error {
	options := workflow.ActivityOptions{
		ScheduleToCloseTimeout: 30 * time.Minute,
		HeartbeatTimeout:       1 * time.Minute,
	}
	ctx = workflow.WithActivityOptions(ctx, options)

	sessionOptions := &workflow.SessionOptions{
		CreationTimeout:  time.Minute,
		ExecutionTimeout: 30 * time.Minute,
	}
	ctx, err := workflow.CreateSession(ctx, sessionOptions)
	if err != nil {
		return err
	}
	defer workflow.CompleteSession(ctx)

	runner := newRunner(ctx, request)

	// blocking call
	return runner.Run(ctx)
}

type Runner struct {
	Activities *workflowActivities
	JobRunner  jobRunner
	Request    Request
	Store      *state.WorkflowStore
}

func newRunner(ctx workflow.Context, request Request) *Runner {
	var a *workflowActivities

	// Create a dummy route with the correct jobs path
	route := &mux.Route{}
	route.Path("/jobs/{job-id}")

	cmdStepRunner := cmd.Runner{
		Activity: a,
	}

	parent := workflow.GetInfo(ctx).ParentWorkflowExecution

	return &Runner{
		Activities: a,
		Request:    request,
		JobRunner: job_runner.NewRunner(
			&cmdStepRunner,
			&env.Runner{
				CmdRunner: cmdStepRunner,
			},
		),
		Store: &state.WorkflowStore{
			Notifier: func(s *state.Workflow) error {
				return workflow.SignalExternalWorkflow(ctx, parent.ID, parent.RunID, state.WorkflowStateChangeSignal, s).Get(ctx, nil)
			},
			OutputURLGenerator: &state.OutputURLGenerator{
				URLBuilder: route,
			},
		},
	}
}

func (r *Runner) Plan(ctx workflow.Context, root *root.LocalRoot, serverURL *url.URL) error {
	jobID, err := sideeffect.GenerateUUID(ctx)

	if err != nil {
		return errors.Wrap(err, "generating job id")
	}

	if err := r.Store.InitPlanJob(jobID, serverURL); err != nil {
		return errors.Wrap(err, "initializing job")
	}

	_, err = r.JobRunner.Run(ctx, r.Request.Root.Plan, root)

	if err != nil {
		if e := r.Store.UpdatePlanJobWithStatus(state.FailedJobStatus); e != nil {
			logger.Error(ctx, "unable to update job with failed status, job failed with error. ", err)
			return errors.Wrap(e, "updating job with failed status")
		}
		return errors.Wrap(err, "running job")
	}
	if err := r.Store.UpdatePlanJobWithStatus(state.SuccessJobStatus); err != nil {
		return errors.Wrap(err, "updating job with success status")
	}

	return nil
}

func (r *Runner) Apply(ctx workflow.Context, root *root.LocalRoot, serverURL *url.URL) error {
	jobID, err := sideeffect.GenerateUUID(ctx)

	if err != nil {
		return errors.Wrap(err, "generating job id")
	}

	if err := r.Store.InitApplyJob(jobID, serverURL); err != nil {
		return errors.Wrap(err, "initializing apply job")
	}

	// Wait for plan review signal
	var planReview PlanReview
	signalChan := workflow.GetSignalChannel(ctx, "planreview-repo-steps")
	_ = signalChan.Receive(ctx, &planReview)

	if planReview.Status == Rejected {
		if err := r.Store.UpdateApplyJobWithStatus(state.RejectedJobStatus); err != nil {
			return errors.Wrap(err, "updating apply job with rejected status")
		}
		return nil
	}

	_, err = r.JobRunner.Run(ctx, r.Request.Root.Apply, root)
	if err != nil {

		if err := r.Store.UpdateApplyJobWithStatus(state.FailedJobStatus); err != nil {
			return errors.Wrap(err, "updating apply job with failed status")
		}
		return errors.Wrap(err, "running plan job")
	}

	if err := r.Store.UpdateApplyJobWithStatus(state.SuccessJobStatus); err != nil {
		return errors.Wrap(err, "updating apply job with success status")
	}

	return nil
}

func (r *Runner) Run(ctx workflow.Context) error {
	var response *activities.GetWorkerInfoResponse
	err := workflow.ExecuteActivity(ctx, r.Activities.GetWorkerInfo).Get(ctx, &response)

	if err != nil {
		return errors.Wrap(err, "getting worker info")
	}

	// Clone repository into disk
	var cloneResponse activities.GithubRepoCloneResponse
	err = workflow.ExecuteActivity(ctx, r.Activities.GithubRepoClone, activities.GithubRepoCloneRequest{}).Get(ctx, &cloneResponse)
	if err != nil {
		return errors.Wrap(err, "executing GH repo clone")
	}

	if err := r.Plan(ctx, cloneResponse.LocalRoot, response.ServerURL); err != nil {
		return err
	}

	if err := r.Apply(ctx, cloneResponse.LocalRoot, response.ServerURL); err != nil {
		return err
	}

	// Cleanup
	err = workflow.ExecuteActivity(ctx, r.Activities.Cleanup, activities.CleanupRequest{}).Get(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "cleaning up")
	}
	return nil
}
