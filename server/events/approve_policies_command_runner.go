package events

import (
	"context"
	"fmt"

	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
)

func NewApprovePoliciesCommandRunner(
	commitStatusUpdater CommitStatusUpdater,
	prjCommandBuilder ProjectApprovePoliciesCommandBuilder,
	prjCommandRunner ProjectApprovePoliciesCommandRunner,
	pullUpdater *PullUpdater,
	dbUpdater *DBUpdater,
) *ApprovePoliciesCommandRunner {
	return &ApprovePoliciesCommandRunner{
		commitStatusUpdater: commitStatusUpdater,
		prjCmdBuilder:       prjCommandBuilder,
		prjCmdRunner:        prjCommandRunner,
		pullUpdater:         pullUpdater,
		dbUpdater:           dbUpdater,
	}
}

type ApprovePoliciesCommandRunner struct {
	commitStatusUpdater CommitStatusUpdater
	pullUpdater         *PullUpdater
	dbUpdater           *DBUpdater
	prjCmdBuilder       ProjectApprovePoliciesCommandBuilder
	prjCmdRunner        ProjectApprovePoliciesCommandRunner
}

func (a *ApprovePoliciesCommandRunner) Run(ctx *command.Context, cmd *command.Comment) {
	baseRepo := ctx.Pull.BaseRepo
	pull := ctx.Pull

	var statusId string
	var err error
	statusId, err = a.commitStatusUpdater.UpdateCombined(context.TODO(), baseRepo, pull, models.PendingCommitStatus, command.PolicyCheck, statusId)
	if err != nil {
		ctx.Log.Warnf("unable to update commit status: %s", err)
	}

	projectCmds, err := a.prjCmdBuilder.BuildApprovePoliciesCommands(ctx, cmd)
	if err != nil {
		if _, statusErr := a.commitStatusUpdater.UpdateCombined(context.TODO(), ctx.Pull.BaseRepo, ctx.Pull, models.FailedCommitStatus, command.PolicyCheck, statusId); statusErr != nil {
			ctx.Log.Warnf("unable to update commit status: %s", statusErr)
		}
		a.pullUpdater.UpdatePull(ctx, cmd, command.Result{Error: err})
		return
	}

	if len(projectCmds) == 0 {
		ctx.Log.Infof("determined there was no project to run approve_policies in")
		// If there were no projects modified, we set successful commit statuses
		// with 0/0 projects approve_policies successfully because some users require
		// the Atlantis status to be passing for all pull requests.
		if _, err := a.commitStatusUpdater.UpdateCombinedCount(context.TODO(), ctx.Pull.BaseRepo, ctx.Pull, models.SuccessCommitStatus, command.PolicyCheck, statusId, 0, 0); err != nil {
			ctx.Log.Warnf("unable to update commit status: %s", err)
		}
		return
	}

	result := a.buildApprovePolicyCommandResults(ctx, projectCmds)

	a.pullUpdater.UpdatePull(
		ctx,
		cmd,
		result,
	)

	pullStatus, err := a.dbUpdater.updateDB(ctx, pull, result.ProjectResults)
	if err != nil {
		ctx.Log.Errorf("writing results: %s", err)
		return
	}

	a.updateCommitStatus(ctx, pullStatus, statusId)
}

func (a *ApprovePoliciesCommandRunner) buildApprovePolicyCommandResults(ctx *command.Context, prjCmds []command.ProjectContext) (result command.Result) {
	// Check if vcs user is in the owner list of the PolicySets. All projects
	// share the same Owners list at this time so no reason to iterate over each
	// project.
	if len(prjCmds) > 0 && !prjCmds[0].PolicySets.IsOwner(ctx.User.Username) {
		result.Error = fmt.Errorf("contact policy owners to approve failing policies")
		return
	}

	var prjResults []command.ProjectResult

	for _, prjCmd := range prjCmds {
		prjResult := a.prjCmdRunner.ApprovePolicies(prjCmd)
		prjResults = append(prjResults, prjResult)
	}
	result.ProjectResults = prjResults
	return
}

func (a *ApprovePoliciesCommandRunner) updateCommitStatus(ctx *command.Context, pullStatus models.PullStatus, statusId string) {
	var numSuccess int
	var numErrored int
	status := models.SuccessCommitStatus

	numSuccess = pullStatus.StatusCount(models.PassedPolicyCheckStatus)
	numErrored = pullStatus.StatusCount(models.ErroredPolicyCheckStatus)

	if numErrored > 0 {
		status = models.FailedCommitStatus
	}

	if _, err := a.commitStatusUpdater.UpdateCombinedCount(context.TODO(), ctx.Pull.BaseRepo, ctx.Pull, status, command.PolicyCheck, statusId, numSuccess, len(pullStatus.Projects)); err != nil {
		ctx.Log.Warnf("unable to update commit status: %s", err)
	}
}
