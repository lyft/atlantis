package events

import (
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
)

// ProjectOutputWrapper is a decorator that creates a new PR status check per project.
// The status contains a url that outputs current progress of the terraform plan/apply command.
type ProjectOutputWrapper struct {
	ProjectCommandRunner
	JobURLSetter JobURLSetter
	JobCloser    JobCloser
}

func (p *ProjectOutputWrapper) Plan(ctx command.ProjectContext) command.ProjectResult {
	result := p.updateProjectPRStatus(command.Plan, ctx, p.ProjectCommandRunner.Plan)
	p.JobCloser.CloseJob(ctx.JobID, ctx.BaseRepo)
	return result
}

func (p *ProjectOutputWrapper) Apply(ctx command.ProjectContext) command.ProjectResult {
	result := p.updateProjectPRStatus(command.Apply, ctx, p.ProjectCommandRunner.Apply)
	p.JobCloser.CloseJob(ctx.JobID, ctx.BaseRepo)
	return result
}

func (p *ProjectOutputWrapper) updateProjectPRStatus(commandName command.Name, ctx command.ProjectContext, execute func(ctx command.ProjectContext) command.ProjectResult) command.ProjectResult {
	// Create a PR status to track project's plan status. The status will
	// include a link to view the progress of atlantis plan command in real
	// time
	var statusId string
	var err error
	statusId, err = p.JobURLSetter.SetJobURLWithStatus(ctx, commandName, models.PendingCommitStatus, statusId)
	if err != nil {
		ctx.Log.Errorf("updating project PR status", err)
	}

	// ensures we are differentiating between project level command and overall command
	result := execute(ctx)

	if result.Error != nil || result.Failure != "" {
		if _, err := p.JobURLSetter.SetJobURLWithStatus(ctx, commandName, models.FailedCommitStatus, statusId); err != nil {
			ctx.Log.Errorf("updating project PR status", err)
		}

		return result
	}

	if _, err := p.JobURLSetter.SetJobURLWithStatus(ctx, commandName, models.SuccessCommitStatus, statusId); err != nil {
		ctx.Log.Errorf("updating project PR status", err)
	}

	return result
}
