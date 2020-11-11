package events

import (
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
)

func NewPlanCommandRunner(
	commandRunner *DefaultCommandRunner,
	isAutoplan bool,
) *planCommandRunner {
	return &planCommandRunner{
		isAutoplan:                isAutoplan,
		commandRunner:             commandRunner,
		silenceVCSStatusNoPlans:   commandRunner.SilenceVCSStatusNoPlans,
		globalAutomerge:           commandRunner.GlobalAutomerge,
		vcsClient:                 commandRunner.VCSClient,
		commitStatusUpdater:       commandRunner.CommitStatusUpdater,
		prjCmdBuilderFunc:         commandRunner.ProjectCommandBuilder.BuildPlanCommands,
		prjCmdAutoplanBuilderFunc: commandRunner.ProjectCommandBuilder.BuildAutoplanCommands,
		prjCmdRunnerFunc:          commandRunner.ProjectCommandRunner.Plan,
	}
}

type planCommandRunner struct {
	commandRunner             *DefaultCommandRunner
	vcsClient                 vcs.Client
	globalAutomerge           bool
	isAutoplan                bool
	silenceVCSStatusNoPlans   bool
	commitStatusUpdater       CommitStatusUpdater
	prjCmdBuilderFunc         cmdBuilderFunc
	prjCmdRunnerFunc          cmdRunnerFunc
	prjCmdAutoplanBuilderFunc cmdAutoplanBuilderFunc
}

func (p *planCommandRunner) runAutoplan(ctx *CommandContext) {
	baseRepo := ctx.Pull.BaseRepo
	pull := ctx.Pull

	projectCmds, err := p.prjCmdAutoplanBuilderFunc(ctx)
	if err != nil {
		if statusErr := p.commitStatusUpdater.UpdateCombined(baseRepo, pull, models.FailedCommitStatus, models.PlanCommand); statusErr != nil {
			ctx.Log.Warn("unable to update commit status: %s", statusErr)
		}
		p.commandRunner.updatePull(ctx, AutoplanCommand{}, CommandResult{Error: err})
		return
	}

	projectCmds, policyCheckCmds := p.partitionProjectCmds(ctx, projectCmds)

	if len(projectCmds) == 0 {
		ctx.Log.Info("determined there was no project to run plan in")
		if !p.silenceVCSStatusNoPlans {
			// If there were no projects modified, we set a successful commit status
			// with 0/0 projects planned successfully because some users require
			// the Atlantis status to be passing for all pull requests.
			ctx.Log.Debug("setting VCS status to success with no projects found")
			if err := p.commitStatusUpdater.UpdateCombinedCount(baseRepo, pull, models.SuccessCommitStatus, models.PlanCommand, 0, 0); err != nil {
				ctx.Log.Warn("unable to update commit status: %s", err)
			}
		}
		return
	}

	// At this point we are sure Atlantis has work to do, so set commit status to pending
	if err := p.commitStatusUpdater.UpdateCombined(ctx.Pull.BaseRepo, ctx.Pull, models.PendingCommitStatus, models.PlanCommand); err != nil {
		ctx.Log.Warn("unable to update commit status: %s", err)
	}

	// Only run commands in parallel if enabled
	var result CommandResult
	if p.isParallelEnabled(ctx, projectCmds) {
		ctx.Log.Info("Running plans in parallel")
		result = runProjectCmdsParallel(projectCmds, p.prjCmdRunnerFunc)
	} else {
		result = runProjectCmds(projectCmds, p.prjCmdRunnerFunc)
	}

	if p.automergeEnabled(projectCmds) && result.HasErrors() {
		ctx.Log.Info("deleting plans because there were errors and automerge requires all plans succeed")
		p.deletePlans(ctx)
		result.PlansDeleted = true
	}

	p.commandRunner.updatePull(ctx, AutoplanCommand{}, result)

	pullStatus, err := p.commandRunner.updateDB(ctx, ctx.Pull, result.ProjectResults)
	if err != nil {
		p.commandRunner.Logger.Err("writing results: %s", err)
	}

	p.updateCommitStatus(ctx, pullStatus)

	// Check if there are any planned projects and if there are any errors or if plans are being deleted
	if len(policyCheckCmds) > 0 &&
		!(result.HasErrors() || result.PlansDeleted) {
		// Run policy_check command
		ctx.Log.Info("Running policy_checks for all plans")
		pcCmdRunner := NewPolicyCheckCommandRunner(
			p.commandRunner,
			p.commitStatusUpdater,
		)
		pcCmdRunner.Run(ctx, policyCheckCmds)
	}
}

func (p *planCommandRunner) run(ctx *CommandContext, cmd *CommentCommand) {
	var err error
	baseRepo := ctx.Pull.BaseRepo
	pull := ctx.Pull

	if err = p.commitStatusUpdater.UpdateCombined(baseRepo, pull, models.PendingCommitStatus, models.PlanCommand); err != nil {
		ctx.Log.Warn("unable to update commit status: %s", err)
	}

	projectCmds, err := p.prjCmdBuilderFunc(ctx, cmd)
	if err != nil {
		if statusErr := p.commitStatusUpdater.UpdateCombined(ctx.Pull.BaseRepo, ctx.Pull, models.FailedCommitStatus, models.PlanCommand); statusErr != nil {
			ctx.Log.Warn("unable to update commit status: %s", statusErr)
		}
		p.commandRunner.updatePull(ctx, cmd, CommandResult{Error: err})
		return
	}

	projectCmds, policyCheckCmds := p.partitionProjectCmds(ctx, projectCmds)

	// Only run commands in parallel if enabled
	var result CommandResult
	if p.isParallelEnabled(ctx, projectCmds) {
		ctx.Log.Info("Running applies in parallel")
		result = runProjectCmdsParallel(projectCmds, p.prjCmdRunnerFunc)
	} else {
		result = runProjectCmds(projectCmds, p.prjCmdRunnerFunc)
	}

	if p.automergeEnabled(projectCmds) && result.HasErrors() {
		ctx.Log.Info("deleting plans because there were errors and automerge requires all plans succeed")
		p.deletePlans(ctx)
		result.PlansDeleted = true
	}

	p.commandRunner.updatePull(
		ctx,
		cmd,
		result)

	pullStatus, err := p.commandRunner.updateDB(ctx, pull, result.ProjectResults)
	if err != nil {
		p.commandRunner.Logger.Err("writing results: %s", err)
		return
	}

	p.updateCommitStatus(ctx, pullStatus)

	// Runs policy checks step after all plans are successful.
	// This step does not approve any policies that require approval.
	if len(result.ProjectResults) > 0 &&
		!(result.HasErrors() || result.PlansDeleted) {
		ctx.Log.Info("Running policy check for %s", cmd.String())
		pcCmdRunner := NewPolicyCheckCommandRunner(
			p.commandRunner,
			p.commitStatusUpdater,
		)
		pcCmdRunner.Run(ctx, policyCheckCmds)
	}
}

func (p *planCommandRunner) Run(ctx *CommandContext, cmd *CommentCommand) {
	if p.isAutoplan {
		p.runAutoplan(ctx)
	} else {
		p.run(ctx, cmd)
	}
}

func (p *planCommandRunner) updateCommitStatus(ctx *CommandContext, pullStatus models.PullStatus) {
	var numSuccess int
	var numErrored int
	status := models.SuccessCommitStatus

	numErrored = pullStatus.StatusCount(models.ErroredPlanStatus)
	// We consider anything that isn't a plan error as a plan success.
	// For example, if there is an apply error, that means that at least a
	// plan was generated successfully.
	numSuccess = len(pullStatus.Projects) - numErrored

	if numErrored > 0 {
		status = models.FailedCommitStatus
	}

	if err := p.commitStatusUpdater.UpdateCombinedCount(
		ctx.Pull.BaseRepo,
		ctx.Pull,
		status,
		models.PlanCommand,
		numSuccess,
		len(pullStatus.Projects),
	); err != nil {
		ctx.Log.Warn("unable to update commit status: %s", err)
	}
}

// deletePlans deletes all plans generated in this ctx.
func (p *planCommandRunner) deletePlans(ctx *CommandContext) {
	pullDir, err := p.commandRunner.WorkingDir.GetPullDir(ctx.Pull.BaseRepo, ctx.Pull)
	if err != nil {
		ctx.Log.Err("getting pull dir: %s", err)
	}
	if err := p.commandRunner.PendingPlanFinder.DeletePlans(pullDir); err != nil {
		ctx.Log.Err("deleting pending plans: %s", err)
	}
}

// automergeEnabled returns true if automerging is enabled in this context.
func (p *planCommandRunner) automergeEnabled(projectCmds []models.ProjectCommandContext) bool {
	// If the global automerge is set, we always automerge.
	return p.globalAutomerge ||
		// Otherwise we check if this repo is configured for automerging.
		(len(projectCmds) > 0 && projectCmds[0].AutomergeEnabled)
}

func (p *planCommandRunner) partitionProjectCmds(
	ctx *CommandContext,
	cmds []models.ProjectCommandContext,
) (
	projectCmds []models.ProjectCommandContext,
	policyCheckCmds []models.ProjectCommandContext,
) {
	for _, cmd := range cmds {
		switch cmd.CommandName {
		case models.PlanCommand:
			projectCmds = append(projectCmds, cmd)
		case models.PolicyCheckCommand:
			policyCheckCmds = append(policyCheckCmds, cmd)
		default:
			ctx.Log.Err("%s is not supported", cmd.CommandName)
		}
	}
	return
}

func (a *planCommandRunner) isParallelEnabled(ctx *CommandContext, projectCmds []models.ProjectCommandContext) bool {
	return len(projectCmds) > 0 && projectCmds[0].ParallelPlanEnabled
}
