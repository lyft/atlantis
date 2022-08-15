package terraform

import (
	"context"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/steps"
	"go.temporal.io/sdk/workflow"
	"time"
)

const TaskQueue = "terraform"

type PlanStatus int

const (
	Approved PlanStatus = iota
	Rejected
)

func Workflow(ctx workflow.Context, request Request) error {
	options := workflow.ActivityOptions{
		TaskQueue:              TaskQueue,
		ScheduleToCloseTimeout: 30 * time.Minute,
		HeartbeatTimeout:       1 * time.Minute,
	}
	ctx = workflow.WithActivityOptions(ctx, options)

	sessionOptions := &workflow.SessionOptions{
		CreationTimeout:  time.Minute,
		ExecutionTimeout: time.Minute,
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
	ExecuteCommand(context.Context, activities.ExecuteCommandRequest) error
	Notify(context.Context, activities.NotifyRequest) error
	Cleanup(context.Context, activities.CleanupRequest) error
}

type Runner struct {
	workerActivities
	request Request
}

func newRunner(ctx workflow.Context, request Request) *Runner {
	return &Runner{
		workerActivities: activities.Terraform{},
		request:          request,
	}
}

func (r *Runner) Run(ctx workflow.Context) error {
	// Clone repository into disk
	err := workflow.ExecuteActivity(ctx, r.GithubRepoClone, activities.GithubRepoCloneRequest{}).Get(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "executing GH repo clone")
	}
	// Run plan steps
	for _, step := range r.request.Root.Plan.Steps {
		err := r.runStep(ctx, step)
		if err != nil {
			return errors.Wrap(err, "running step")
		}
	}
	// Wait for plan review signal
	var planReview PlanReview
	signalChan := workflow.GetSignalChannel(ctx, "planreview-repo-steps")
	signalChan.Receive(ctx, &planReview)
	if planReview.Status == Rejected {
		return nil
	}
	// Run apply steps
	for _, step := range r.request.Root.Apply.Steps {
		err := r.runStep(ctx, step)
		if err != nil {
			return errors.Wrap(err, "running step")
		}
	}
	// Cleanup
	err = workflow.ExecuteActivity(ctx, r.Cleanup, activities.CleanupRequest{}).Get(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "cleaning up")
	}
	return nil
}

// TODO: wrap each case statement's ExecuteActivity with a specific Runner implementation;
// activity itself should just handle the non-deterministic code chunk (i.e. running terraform operation, state updates)
// ex:
//type Runner interface {
//	Run(ctx workflow.Context, step deploy.Step) error
//}
//case "init":
//	err = initStepRunner.Run(ctx, step)
//	if err != nil {
//		return errors.Wrap(err, "executing terraform init")
//	}
func (r *Runner) runStep(ctx workflow.Context, step steps.Step) error {
	var err error
	switch step.StepName {
	case "init":
		err = workflow.ExecuteActivity(ctx, r.TerraformInit, activities.TerraformInitRequest{}).Get(ctx, nil)
		if err != nil {
			return errors.Wrap(err, "executing terraform init")
		}
	case "plan":
		err = workflow.ExecuteActivity(ctx, r.TerraformPlan, activities.TerraformPlanRequest{}).Get(ctx, nil)
		if err != nil {
			return errors.Wrap(err, "executing terraform plan")
		}
		err = workflow.ExecuteActivity(ctx, r.Notify, activities.NotifyRequest{}).Get(ctx, nil)
		if err != nil {
			return errors.Wrap(err, "notifying plan result")
		}
	case "apply":
		err = workflow.ExecuteActivity(ctx, r.TerraformApply, activities.TerraformApplyRequest{}).Get(ctx, nil)
		if err != nil {
			return errors.Wrap(err, "executing terraform apply")
		}
		err = workflow.ExecuteActivity(ctx, r.Notify, activities.NotifyRequest{}).Get(ctx, nil)
		if err != nil {
			return errors.Wrap(err, "notifying apply result")
		}
	case "run":
		err = workflow.ExecuteActivity(ctx, r.ExecuteCommand, activities.ExecuteCommandRequest{}).Get(ctx, nil)
		if err != nil {
			return errors.Wrap(err, "executing custom command")
		}
	case "env":
		err = workflow.ExecuteActivity(ctx, r.ExecuteCommand, activities.ExecuteCommandRequest{}).Get(ctx, nil)
		if err != nil {
			return errors.Wrap(err, "exporting custom env variables")
		}
	}
	if err != nil {
		return err
	}
	return nil
}

type PlanReview struct {
	Status PlanStatus
}
