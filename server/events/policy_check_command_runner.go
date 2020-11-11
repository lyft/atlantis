package events

import "github.com/runatlantis/atlantis/server/events/models"

func NewPolicyCheckCommandRunner(
	cmdRunner *DefaultCommandRunner,
	commitStatusUpdater CommitStatusUpdater,
) *policyCheckCommandRunner {
	return &policyCheckCommandRunner{
		cmdRunner:           cmdRunner,
		commitStatusUpdater: commitStatusUpdater,
		prjCmdRunnerFunc:    cmdRunner.ProjectCommandRunner.PolicyCheck,
	}
}

type policyCheckCommandRunner struct {
	cmdRunner           *DefaultCommandRunner
	ctx                 *CommandContext
	commitStatusUpdater CommitStatusUpdater
	prjCmdRunnerFunc    cmdRunnerFunc
}

func (p *policyCheckCommandRunner) Run(ctx *CommandContext, projectCmds []models.ProjectCommandContext) {
	if len(projectCmds) == 0 {
		return
	}

	// So set policy_check commit status to pending
	if err := p.commitStatusUpdater.UpdateCombined(ctx.Pull.BaseRepo, ctx.Pull, models.PendingCommitStatus, models.PolicyCheckCommand); err != nil {
		ctx.Log.Warn("unable to update commit status: %s", err)
	}

	var result CommandResult
	if p.isParallelEnabled(projectCmds) {
		ctx.Log.Info("Running policy_checks in parallel")
		result = runProjectCmdsParallel(projectCmds, p.prjCmdRunnerFunc)
	} else {
		result = runProjectCmds(projectCmds, p.prjCmdRunnerFunc)
	}

	p.cmdRunner.updatePull(ctx, PolicyCheckCommand{}, result)

	pullStatus, err := p.cmdRunner.updateDB(ctx, ctx.Pull, result.ProjectResults)
	if err != nil {
		p.cmdRunner.Logger.Err("writing results: %s", err)
	}

	p.updateCommitStatus(ctx, pullStatus)
}

func (p *policyCheckCommandRunner) updateCommitStatus(ctx *CommandContext, pullStatus models.PullStatus) {
	var numSuccess int
	var numErrored int
	status := models.SuccessCommitStatus

	numSuccess = pullStatus.StatusCount(models.PassedPolicyCheckStatus)
	numErrored = pullStatus.StatusCount(models.ErroredPolicyCheckStatus)

	if numErrored > 0 {
		status = models.FailedCommitStatus
	}

	if err := p.commitStatusUpdater.UpdateCombinedCount(ctx.Pull.BaseRepo, ctx.Pull, status, models.PolicyCheckCommand, numSuccess, len(pullStatus.Projects)); err != nil {
		ctx.Log.Warn("unable to update commit status: %s", err)
	}
}

func (a *policyCheckCommandRunner) isParallelEnabled(projectCmds []models.ProjectCommandContext) bool {
	return len(projectCmds) > 0 && projectCmds[0].ParallelPolicyCheckEnabled
}
