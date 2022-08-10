package events

import (
	"context"
	"fmt"

	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/lyft/feature"
)

type ChecksEnabledApprovePoliciesCommandRunner struct {
	command.Runner

	FeatureAllocator feature.Allocator
}

func (c *ChecksEnabledApprovePoliciesCommandRunner) Run(ctx *command.Context, cmd *command.Comment) {
	shouldAllocate, err := c.FeatureAllocator.ShouldAllocate(feature.GithubChecks, feature.FeatureContext{
		RepoName:         ctx.HeadRepo.FullName,
		PullCreationTime: ctx.Pull.CreatedAt,
	})
	if err != nil {
		ctx.Log.ErrorContext(ctx.RequestCtx, fmt.Sprintf("unable to allocate for feature: %s, error: %s", feature.GithubChecks, err))
	}

	// Turn the flag on if github checks is enabled
	if shouldAllocate {
		cmd.GithubChecksEnabled = true
	}

	c.Runner.Run(ctx, cmd)
}

func NewChecksEnabledApprovePoliciesCommandRunner(runnner command.Runner, featureAllocator feature.Allocator) command.Runner {
	return &ChecksEnabledApprovePoliciesCommandRunner{
		Runner:           runnner,
		FeatureAllocator: featureAllocator,
	}
}

func NewApprovePoliciesCommandRunner(
	commitStatusUpdater CommitStatusUpdater,
	prjCommandBuilder ProjectApprovePoliciesCommandBuilder,
	prjCommandRunner ProjectApprovePoliciesCommandRunner,
	outputUpdater OutputUpdater,
	dbUpdater *DBUpdater,
	projectPolicyCheckCommandRunner ProjectPolicyCheckCommandRunner,
	projectCommandBuilder ProjectPlanCommandBuilder,
	logger logging.Logger,
) *ApprovePoliciesCommandRunner {
	return &ApprovePoliciesCommandRunner{
		commitStatusUpdater:             commitStatusUpdater,
		prjCmdBuilder:                   prjCommandBuilder,
		prjCmdRunner:                    prjCommandRunner,
		outputUpdater:                   outputUpdater,
		dbUpdater:                       dbUpdater,
		projectPolicyCheckCommandRunner: projectPolicyCheckCommandRunner,
		projectCommandBuilder:           projectCommandBuilder,
		Logger:                          logger,
	}
}

type ApprovePoliciesCommandRunner struct {
	commitStatusUpdater             CommitStatusUpdater
	outputUpdater                   OutputUpdater
	dbUpdater                       *DBUpdater
	prjCmdBuilder                   ProjectApprovePoliciesCommandBuilder
	prjCmdRunner                    ProjectApprovePoliciesCommandRunner
	projectPolicyCheckCommandRunner ProjectPolicyCheckCommandRunner
	projectCommandBuilder           ProjectPlanCommandBuilder
	Logger                          logging.Logger
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

	result := a.buildApprovePolicyCommandResults(ctx, projectCmds)

	pullStatus, err := a.dbUpdater.updateDB(ctx, pull, result.ProjectResults)
	if err != nil {
		ctx.Log.ErrorContext(ctx.RequestCtx, fmt.Sprintf("writing results: %s", err))
		return
	}

	// Rerun policy check and populate output since github checks does not retain the output
	if cmd.GithubChecksEnabled {
		a.runPolicyCheckAndPopulateOuptut(ctx, cmd, &result)
	}

	a.outputUpdater.UpdateOutput(
		ctx,
		cmd,
		result,
	)

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

func (a *ApprovePoliciesCommandRunner) runPolicyCheckAndPopulateOuptut(ctx *command.Context, cmd *command.Comment, result *command.Result) {
	a.Logger.Info("building policy check context")
	prjCmds, err := a.projectCommandBuilder.BuildPlanCommands(ctx, &command.Comment{
		RepoRelDir:    cmd.RepoRelDir,
		Name:          command.Plan,
		Workspace:     cmd.Workspace,
		ProjectName:   cmd.ProjectName,
		LogLevel:      cmd.LogLevel,
		SkipCheckRuns: true,
	})

	for _, prjCmd := range prjCmds {
		a.Logger.Info("project command", map[string]interface{}{
			"project commnand": prjCmd,
		})
	}

	if err != nil {
		a.Logger.Error("build plan command for policy approval failed")
	}

	_, policyCheckCommands := a.partitionProjectCmds(ctx, prjCmds)
	for _, prjCmd := range policyCheckCommands {
		a.Logger.Info("Policy Check Command", map[string]interface{}{
			"policy check": prjCmd,
		})
	}

	policyCheckOutput := map[string]string{}
	for _, policyCheckCommand := range policyCheckCommands {
		res := a.projectPolicyCheckCommandRunner.PolicyCheck(policyCheckCommand)

		// Skip if policy check is success
		if res.Failure == "" {
			continue
		}

		a.Logger.Info("Result: ", map[string]interface{}{
			"Result": res,
		})
		policyCheckOutput[policyCheckCommand.ProjectName] = res.Failure
	}
	a.Logger.Info("Policy Check Output", map[string]interface{}{
		"output": policyCheckOutput,
	})

	// Populate the results with the policy check output
	for i, prjResult := range result.ProjectResults {
		if _, ok := policyCheckOutput[prjResult.ProjectName]; !ok {
			continue
		}

		result.ProjectResults[i].Failure = policyCheckOutput[prjResult.ProjectName]

		// To avoid populating output from the approve policies command
		result.ProjectResults[i].PolicyCheckSuccess = nil
	}

	a.Logger.Info("approve policies result", map[string]interface{}{
		"approve policies": result,
	})
}
