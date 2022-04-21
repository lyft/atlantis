package events

import (
	"context"
	"fmt"

	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
)

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_project_job_url_generator.go ProjectJobURLGenerator

// ProjectJobURLGenerator generates urls to view project's progress.
type ProjectJobURLGenerator interface {
	GenerateProjectJobURL(p command.ProjectContext) (string, error)
}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_project_status_updater.go ProjectStatusUpdater

type ProjectStatusUpdater interface {
	// UpdateProject sets the commit status for the project represented by
	// ctx.
	UpdateProject(ctx context.Context, projectCtx command.ProjectContext, cmdName fmt.Stringer, status models.CommitStatus, url string) (string, error)
}

// JobsEnabledProjectCommandRunner is a decorator that creates a new PR status check per project.
// The status contains a url that outputs current progress of the terraform plan/apply command.
type JobsEnabledProjectCommandRunner struct {
	ProjectCommandRunner

	JobUrlGenerator ProjectJobURLGenerator
	StatusUpdater   CommitStatusUpdater
	JobCloser       JobCloser
}

func (p *JobsEnabledProjectCommandRunner) Plan(ctx command.ProjectContext) command.ProjectResult {
	// generate job URL and status Id
	url, _ := p.JobUrlGenerator.GenerateProjectJobURL(ctx)
	statusId, _ := p.StatusUpdater.UpdateProject(context.TODO(), ctx, ctx.CommandName, models.PendingCommitStatus, url, "")

	// Store check run id to update the check run when the operation is complete
	ctx.CheckRunId = statusId

	result := p.ProjectCommandRunner.Plan(ctx)

	// Update status to failed
	if result.Error != nil || result.Failure != "" {
		if _, err := p.StatusUpdater.UpdateProject(context.TODO(), ctx, ctx.CommandName, models.FailedCommitStatus, url, ""); err != nil {
			ctx.Log.Errorf("updating project PR status", err)
		}

		return result
	}

	// Update status to success with the terraform output
	if _, err := p.StatusUpdater.UpdateProject(context.TODO(), ctx, ctx.CommandName, models.SuccessCommitStatus, url, result.PlanSuccess.TerraformOutput); err != nil {
		ctx.Log.Errorf("updating project PR status", err)
	}

	// Close job
	p.JobCloser.CloseJob(ctx.JobID, ctx.BaseRepo)
	return result
}

func (p *JobsEnabledProjectCommandRunner) Apply(ctx command.ProjectContext) command.ProjectResult {
	// generate job URL and status Id
	url, _ := p.JobUrlGenerator.GenerateProjectJobURL(ctx)
	statusId, _ := p.StatusUpdater.UpdateProject(context.TODO(), ctx, ctx.CommandName, models.PendingCommitStatus, url, "")

	// Store check run id to update the check run when the operation is complete
	ctx.CheckRunId = statusId

	result := p.ProjectCommandRunner.Apply(ctx)

	// Update status to failed
	if result.Error != nil || result.Failure != "" {
		if _, err := p.StatusUpdater.UpdateProject(context.TODO(), ctx, ctx.CommandName, models.FailedCommitStatus, url, ""); err != nil {
			ctx.Log.Errorf("updating project PR status", err)
		}

		return result
	}

	// Update status to success with the terraform output
	if _, err := p.StatusUpdater.UpdateProject(context.TODO(), ctx, ctx.CommandName, models.SuccessCommitStatus, url, result.ApplySuccess); err != nil {
		ctx.Log.Errorf("updating project PR status", err)
	}

	// Close job
	p.JobCloser.CloseJob(ctx.JobID, ctx.BaseRepo)
	return result
}

func (p *JobsEnabledProjectCommandRunner) PolicyCheck(ctx command.ProjectContext) command.ProjectResult {
	statusId, _ := p.StatusUpdater.UpdateProject(context.TODO(), ctx, ctx.CommandName, models.PendingCommitStatus, "", "")

	ctx.CheckRunId = statusId

	result := p.ProjectCommandRunner.PolicyCheck(ctx)

	if result.Error != nil || result.Failure != "" {
		if _, err := p.StatusUpdater.UpdateProject(context.TODO(), ctx, ctx.CommandName, models.FailedCommitStatus, "", ""); err != nil {
			ctx.Log.Errorf("updating project PR status", err)
		}

		return result
	}

	// Update status to success with the terraform output
	if _, err := p.StatusUpdater.UpdateProject(context.TODO(), ctx, ctx.CommandName, models.SuccessCommitStatus, "", result.PolicyCheckSuccess.PolicyCheckOutput); err != nil {
		ctx.Log.Errorf("updating project PR status", err)
	}
	return result
}
