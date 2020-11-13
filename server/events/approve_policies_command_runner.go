package events

import (
	"fmt"
	"strings"

	"github.com/runatlantis/atlantis/server/events/models"
)

func NewApprovePoliciesCommandRunner(
	cmdRunner *DefaultCommandRunner,
) *ApprovePoliciesCommandRunner {
	return &ApprovePoliciesCommandRunner{
		cmdRunner:           cmdRunner,
		commitStatusUpdater: cmdRunner.CommitStatusUpdater,
		prjCmdBuilder:       cmdRunner.ProjectCommandBuilder,
		prjCmdRunner:        cmdRunner.ProjectCommandRunner,
		policyApprovers:     cmdRunner.PolicyApprovers,
	}
}

type ApprovePoliciesCommandRunner struct {
	cmdRunner           *DefaultCommandRunner
	commitStatusUpdater CommitStatusUpdater
	prjCmdBuilder       ProjectApprovePoliciesCommandBuilder
	prjCmdRunner        ProjectPolicyCheckCommandRunner
	policyApprovers     []string
}

func (a *ApprovePoliciesCommandRunner) Run(ctx *CommandContext, cmd *CommentCommand) {
	baseRepo := ctx.Pull.BaseRepo
	pull := ctx.Pull

	if err := a.commitStatusUpdater.UpdateCombined(baseRepo, pull, models.PendingCommitStatus, models.PolicyCheckCommand); err != nil {
		ctx.Log.Warn("unable to update commit status: %s", err)
	}

	projectCmds, err := a.prjCmdBuilder.BuildApprovePoliciesCommands(ctx, cmd)
	if err != nil {
		if statusErr := a.commitStatusUpdater.UpdateCombined(ctx.Pull.BaseRepo, ctx.Pull, models.FailedCommitStatus, models.PolicyCheckCommand); statusErr != nil {
			ctx.Log.Warn("unable to update commit status: %s", statusErr)
		}
		a.cmdRunner.updatePull(ctx, cmd, CommandResult{Error: err})
		return
	}

	var result CommandResult
	if a.isApprover(ctx.User.Username) {
		result = CommandResult{
			ProjectResults: a.buildProjectResults(ctx, projectCmds),
		}
	} else {
		result = CommandResult{
			Error: fmt.Errorf("Only %s can approve policies.", strings.Join(a.policyApprovers, ",")),
		}
	}

	a.cmdRunner.updatePull(
		ctx,
		cmd,
		result,
	)

	pullStatus, err := a.cmdRunner.updateDB(ctx, pull, result.ProjectResults)
	if err != nil {
		a.cmdRunner.Logger.Err("writing results: %s", err)
		return
	}

	a.updateCommitStatus(ctx, pullStatus)
}

func (a *ApprovePoliciesCommandRunner) isApprover(username string) bool {
	for _, approver := range a.policyApprovers {
		if approver == username {
			return true
		}
	}
	return false
}

func (a *ApprovePoliciesCommandRunner) buildProjectResults(ctx *CommandContext, prjCmds []models.ProjectCommandContext) (prjResults []models.ProjectResult) {
	for _, prjCmd := range prjCmds {
		prjResult := models.ProjectResult{
			Command: models.PolicyCheckCommand,
			PolicyCheckSuccess: &models.PolicyCheckSuccess{
				PolicyCheckOutput: "Policies approved",
			},
			RepoRelDir:  prjCmd.RepoRelDir,
			Workspace:   prjCmd.Workspace,
			ProjectName: prjCmd.ProjectName,
		}
		prjResults = append(prjResults, prjResult)
	}
	return
}

func (a *ApprovePoliciesCommandRunner) updateCommitStatus(ctx *CommandContext, pullStatus models.PullStatus) {
	var numSuccess int
	var numErrored int
	status := models.SuccessCommitStatus

	numSuccess = pullStatus.StatusCount(models.PassedPolicyCheckStatus)
	numErrored = pullStatus.StatusCount(models.ErroredPolicyCheckStatus)

	if numErrored > 0 {
		status = models.FailedCommitStatus
	}

	ctx.Log.Info("status: %+v, numSuccess: %d", pullStatus, numSuccess)
	if err := a.commitStatusUpdater.UpdateCombinedCount(ctx.Pull.BaseRepo, ctx.Pull, status, models.PolicyCheckCommand, numSuccess, len(pullStatus.Projects)); err != nil {
		ctx.Log.Warn("unable to update commit status: %s", err)
	}
}
