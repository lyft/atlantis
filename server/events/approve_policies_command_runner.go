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
	outputUpdater OutputUpdater,
	dbUpdater *DBUpdater,
	policyCheckCommandRunner PolicyCheckCommandRunner,
	projectCommandBuilder ProjectCommandBuilder,
) *ApprovePoliciesCommandRunner {
	return &ApprovePoliciesCommandRunner{
		commitStatusUpdater:      commitStatusUpdater,
		prjCmdBuilder:            prjCommandBuilder,
		prjCmdRunner:             prjCommandRunner,
		outputUpdater:            outputUpdater,
		dbUpdater:                dbUpdater,
		policyCheckCommandRunner: policyCheckCommandRunner,
		projectCommandBuilder:    projectCommandBuilder,
	}
}

type ApprovePoliciesCommandRunner struct {
	commitStatusUpdater      CommitStatusUpdater
	outputUpdater            OutputUpdater
	dbUpdater                *DBUpdater
	prjCmdBuilder            ProjectApprovePoliciesCommandBuilder
	prjCmdRunner             ProjectApprovePoliciesCommandRunner
	policyCheckCommandRunner PolicyCheckCommandRunner
	projectCommandBuilder    ProjectCommandBuilder
}

func (p *ApprovePoliciesCommandRunner) partitionProjectCmds(
	ctx *command.Context,
	cmds []command.ProjectContext,
) (
	projectCmds []command.ProjectContext,
	policyCheckCmds []command.ProjectContext,
) {
	for _, cmd := range cmds {
		switch cmd.CommandName {
		case command.Plan:
			projectCmds = append(projectCmds, cmd)
		case command.PolicyCheck:
			policyCheckCmds = append(policyCheckCmds, cmd)
		default:
			ctx.Log.ErrorContext(ctx.RequestCtx, fmt.Sprintf("%s is not supported", cmd.CommandName))
		}
	}
	return
}

func (a *ApprovePoliciesCommandRunner) Run(ctx *command.Context, cmd *command.Comment) {
	baseRepo := ctx.Pull.BaseRepo
	pull := ctx.Pull

	statusId, err := a.commitStatusUpdater.UpdateCombined(context.TODO(), baseRepo, pull, models.PendingCommitStatus, command.PolicyCheck, "")
	if err != nil {
		ctx.Log.WarnContext(ctx.RequestCtx, fmt.Sprintf("unable to update commit status: %s", err))
	}

	projectCmds, err := a.prjCmdBuilder.BuildApprovePoliciesCommands(ctx, cmd)
	if err != nil {
		if _, statusErr := a.commitStatusUpdater.UpdateCombined(context.TODO(), ctx.Pull.BaseRepo, ctx.Pull, models.FailedCommitStatus, command.PolicyCheck, statusId); statusErr != nil {
			ctx.Log.WarnContext(ctx.RequestCtx, fmt.Sprintf("unable to update commit status: %s", statusErr))
		}
		a.outputUpdater.UpdateOutput(ctx, cmd, command.Result{Error: err})
		return
	}

	if len(projectCmds) == 0 {
		ctx.Log.InfoContext(ctx.RequestCtx, fmt.Sprintf("determined there was no project to run approve_policies in"))
		// If there were no projects modified, we set successful commit statuses
		// with 0/0 projects approve_policies successfully because some users require
		// the Atlantis status to be passing for all pull requests.
		if _, err := a.commitStatusUpdater.UpdateCombinedCount(context.TODO(), ctx.Pull.BaseRepo, ctx.Pull, models.SuccessCommitStatus, command.PolicyCheck, 0, 0, statusId); err != nil {
			ctx.Log.WarnContext(ctx.RequestCtx, fmt.Sprintf("unable to update commit status: %s", err))
		}
		return
	}

	// Build Policy Check Project Context
	ctx.Log.InfoContext(ctx.RequestCtx, "building policy check context")
	prjCmds, err := a.projectCommandBuilder.BuildPlanCommands(ctx, &command.Comment{
		RepoRelDir:  cmd.RepoRelDir,
		Name:        command.Plan,
		Workspace:   cmd.Workspace,
		ProjectName: cmd.ProjectName,
		LogLevel:    cmd.LogLevel,
	})

	for _, prjCmd := range prjCmds {
		ctx.Log.InfoContext(ctx.RequestCtx, "Project Command", map[string]interface{}{
			"project command": prjCmd,
		})
	}

	if err != nil {
		ctx.Log.ErrorContext(ctx.RequestCtx, "build plan command for policy approval failed", map[string]interface{}{})
	}

	_, policyCheckCommands := a.partitionProjectCmds(ctx, prjCmds)
	for _, prjCmd := range policyCheckCommands {
		ctx.Log.InfoContext(ctx.RequestCtx, "Policy Check Command", map[string]interface{}{
			"Policy Check Command": prjCmd,
		})
	}

	policyCheckOutput := map[string]string{}
	for _, policyCheckCommand := range policyCheckCommands {
		res := a.policyCheckCommandRunner.prjCmdRunner.PolicyCheck(policyCheckCommand)
		policyCheckOutput[policyCheckCommand.ProjectName] = res.PolicyCheckSuccess.PolicyCheckOutput
	}

	ctx.Log.InfoContext(ctx.RequestCtx, "policy check output", map[string]interface{}{
		"output": policyCheckOutput,
	})

	result := a.buildApprovePolicyCommandResults(ctx, projectCmds)

	// Populate the results with the policy check output
	for i, prjResult := range result.ProjectResults {
		result.ProjectResults[i].PolicyCheckSuccess.PolicyCheckOutput = policyCheckOutput[prjResult.ProjectName]
	}

	ctx.Log.InfoContext(ctx.RequestCtx, "approve policies result", map[string]interface{}{
		"approve policies": result,
	})

	a.outputUpdater.UpdateOutput(
		ctx,
		cmd,
		result,
	)

	pullStatus, err := a.dbUpdater.updateDB(ctx, pull, result.ProjectResults)
	if err != nil {
		ctx.Log.ErrorContext(ctx.RequestCtx, fmt.Sprintf("writing results: %s", err))
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

	if _, err := a.commitStatusUpdater.UpdateCombinedCount(context.TODO(), ctx.Pull.BaseRepo, ctx.Pull, status, command.PolicyCheck, numSuccess, len(pullStatus.Projects), statusId); err != nil {
		ctx.Log.WarnContext(ctx.RequestCtx, fmt.Sprintf("unable to update commit status: %s", err))
	}
}
