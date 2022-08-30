package terraform

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/job"
	job_runner "github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/job"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/job/step/env"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/job/step/run"
	"go.temporal.io/sdk/workflow"
)

// jobRunner runs a deploy plan/apply job
type jobRunner interface {
	Run(ctx workflow.Context, job job.Job, rootInstance *job.RootInstance) (string, error)
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

type workerActivities interface {
	GithubRepoClone(context.Context, activities.GithubRepoCloneRequest) error
	TerraformInit(context.Context, activities.TerraformInitRequest) error
	TerraformPlan(context.Context, activities.TerraformPlanRequest) error
	TerraformApply(context.Context, activities.TerraformApplyRequest) error
	ExecuteCommand(context.Context, activities.ExecuteCommandRequest) (activities.ExecuteCommandResponse, error)
	Notify(context.Context, activities.NotifyRequest) error
	Cleanup(context.Context, activities.CleanupRequest) error
}

type Runner struct {
	Activities workerActivities
	JobRunner  jobRunner
	Request    Request
}

func newRunner(ctx workflow.Context, request Request) *Runner {
	var a *activities.Terraform

	runStepRunner := run.Runner{
		Activity: a,
	}
	return &Runner{
		Activities: a,
		Request:    request,
		JobRunner: job_runner.NewRunner(
			&runStepRunner,
			&env.Runner{
				RunRunner: runStepRunner,
			},
		),
	}
}

func (r *Runner) Run(ctx workflow.Context) error {
	// Root instance has all the metadata needed to execute a step in a root
	rootInstance := job.BuildRootInstanceFrom(r.Request.Root, r.Request.Repo)

	// Clone repository into disk
	err := workflow.ExecuteActivity(ctx, r.Activities.GithubRepoClone, activities.GithubRepoCloneRequest{}).Get(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "executing GH repo clone")
	}

	_, err = r.JobRunner.Run(ctx, r.Request.Root.Plan, rootInstance)
	if err != nil {
		return errors.Wrap(err, "running step")
	}

	// Wait for plan review signal
	var planReview PlanReview
	signalChan := workflow.GetSignalChannel(ctx, "planreview-repo-steps")
	more := signalChan.Receive(ctx, &planReview)
	if !more {
		return errors.New("plan review signal channel cancelled")
	}
	if planReview.Status == Rejected {
		return nil
	}

	// Run apply steps
	_, err = r.JobRunner.Run(ctx, r.Request.Root.Apply, rootInstance)
	if err != nil {
		return errors.Wrap(err, "running step")
	}

	// Cleanup
	err = workflow.ExecuteActivity(ctx, r.Activities.Cleanup, activities.CleanupRequest{}).Get(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "cleaning up")
	}
	return nil
}
