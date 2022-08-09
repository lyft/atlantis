package terraform

import (
	"context"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"go.temporal.io/sdk/workflow"
	"time"
)

const TaskQueue = "terraform"

func Workflow(ctx workflow.Context, request Request) error {
	options := workflow.ActivityOptions{
		TaskQueue:              TaskQueue,
		ScheduleToCloseTimeout: 30 * time.Minute,
		HeartbeatTimeout:       10 * time.Minute,
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

	// TODO: replace simple runner with
	runner := newRunner(ctx, request)

	// blocking call
	return runner.Run(ctx)
}

type workerActivities interface {
	GithubRepoClone(context.Context, activities.GithubRepoCloneRequest) error
	TerraformPlan(context.Context, activities.TerraformPlanRequest) error
	TerraformApply(context.Context, activities.TerraformApplyRequest) error
	Notify(context.Context, activities.NotifyRequest) error
	Cleanup(context.Context, activities.CleanupRequest) error
}

type Runner struct {
	workerActivities
}

func newRunner(ctx workflow.Context, request Request) *Runner {
	return &Runner{
		workerActivities: activities.Terraform{},
	}
}

func (r *Runner) Run(ctx workflow.Context) error {
	// Clone repository into disk
	err := workflow.ExecuteActivity(ctx, r.GithubRepoClone, activities.GithubRepoCloneRequest{}).Get(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "executing GH repo clone")
	}
	// Run terraform init/plan operation
	err = workflow.ExecuteActivity(ctx, r.TerraformPlan, activities.TerraformPlanRequest{}).Get(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "executing terraform plan")
	}
	// Notify results of preceding terraform operations
	err = workflow.ExecuteActivity(ctx, r.Notify, activities.NotifyRequest{}).Get(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "notifying plan result")
	}
	// Wait for plan review signal
	s := workflow.NewSelector(ctx)
	var planReview bool
	signalChan := workflow.GetSignalChannel(ctx, "")
	s.AddReceive(signalChan, func(c workflow.ReceiveChannel, more bool) {
		c.Receive(ctx, &planReview)
	})
	s.Select(ctx)
	// Run terraform apply operation
	err = workflow.ExecuteActivity(ctx, r.TerraformApply, activities.TerraformApplyRequest{}).Get(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "executing terraform apply")
	}
	// Notify results of preceding terraform operations
	err = workflow.ExecuteActivity(ctx, r.Notify, activities.NotifyRequest{}).Get(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "notifying apply result")
	}
	// Cleanup
	err = workflow.ExecuteActivity(ctx, r.Cleanup, activities.CleanupRequest{}).Get(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "cleaning up")
	}
	return nil
}
