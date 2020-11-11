package events

import (
	"sync"

	"github.com/remeh/sizedwaitgroup"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
)

func NewApplyCommandRunner(cmdRunner *DefaultCommandRunner) *applyCommandRunner {
	return &applyCommandRunner{
		cmdRunner:           cmdRunner,
		vcsClient:           cmdRunner.VCSClient,
		disableApplyAll:     cmdRunner.DisableApplyAll,
		commitStatusUpdater: cmdRunner.CommitStatusUpdater,
		prjCmdBuilderFunc:   cmdRunner.ProjectCommandBuilder.BuildApplyCommands,
		prjCmdRunnerFunc:    cmdRunner.ProjectCommandRunner.Apply,
	}
}

type applyCommandRunner struct {
	cmdRunner           *DefaultCommandRunner
	disableApplyAll     bool
	vcsClient           vcs.Client
	commitStatusUpdater CommitStatusUpdater
	prjCmdBuilderFunc   cmdBuilderFunc
	prjCmdRunnerFunc    cmdRunnerFunc
}

func (a *applyCommandRunner) Run(ctx *CommandContext, cmd *CommentCommand) {
	var err error
	baseRepo := ctx.Pull.BaseRepo
	pull := ctx.Pull

	if a.disableApplyAll && !cmd.IsForSpecificProject() {
		ctx.Log.Info("ignoring apply command without flags since apply all is disabled")
		if err := a.vcsClient.CreateComment(baseRepo, pull.Num, applyAllDisabledComment, models.ApplyCommand.String()); err != nil {
			ctx.Log.Err("unable to comment on pull request: %s", err)
		}

		return
	}

	// Get the mergeable status before we set any build statuses of our own.
	// We do this here because when we set a "Pending" status, if users have
	// required the Atlantis status checks to pass, then we've now changed
	// the mergeability status of the pull request.
	ctx.PullMergeable, err = a.cmdRunner.VCSClient.PullIsMergeable(baseRepo, pull)
	if err != nil {
		// On error we continue the request with mergeable assumed false.
		// We want to continue because not all apply's will need this status,
		// only if they rely on the mergeability requirement.
		ctx.PullMergeable = false
		ctx.Log.Warn("unable to get mergeable status: %s. Continuing with mergeable assumed false", err)
	}
	ctx.Log.Info("pull request mergeable status: %t", ctx.PullMergeable)

	if err = a.commitStatusUpdater.UpdateCombined(baseRepo, pull, models.PendingCommitStatus, cmd.CommandName()); err != nil {
		ctx.Log.Warn("unable to update commit status: %s", err)
	}

	var projectCmds []models.ProjectCommandContext
	projectCmds, err = a.prjCmdBuilderFunc(ctx, cmd)

	if err != nil {
		if statusErr := a.commitStatusUpdater.UpdateCombined(ctx.Pull.BaseRepo, ctx.Pull, models.FailedCommitStatus, cmd.CommandName()); statusErr != nil {
			ctx.Log.Warn("unable to update commit status: %s", statusErr)
		}
		a.cmdRunner.updatePull(ctx, cmd, CommandResult{Error: err})
		return
	}

	// Only run commands in parallel if enabled
	var result CommandResult
	if a.isParallelEnabled(projectCmds) {
		ctx.Log.Info("Running applies in parallel")
		result = runProjectCmdsParallel(projectCmds, a.prjCmdRunnerFunc)
	} else {
		result = runProjectCmds(projectCmds, a.prjCmdRunnerFunc)
	}

	a.cmdRunner.updatePull(
		ctx,
		cmd,
		result)

	pullStatus, err := a.cmdRunner.updateDB(ctx, pull, result.ProjectResults)
	if err != nil {
		a.cmdRunner.Logger.Err("writing results: %s", err)
		return
	}

	a.updateCommitStatus(ctx, pullStatus)

	if a.cmdRunner.automergeEnabled(ctx, projectCmds) {
		a.cmdRunner.automerge(ctx, pullStatus)
	}
}

func (a *applyCommandRunner) isParallelEnabled(projectCmds []models.ProjectCommandContext) bool {
	return len(projectCmds) > 0 && projectCmds[0].ParallelApplyEnabled
}

func (a *applyCommandRunner) updateCommitStatus(ctx *CommandContext, pullStatus models.PullStatus) {
	var numSuccess int
	var numErrored int
	status := models.SuccessCommitStatus

	numSuccess = pullStatus.StatusCount(models.AppliedPlanStatus)
	numErrored = pullStatus.StatusCount(models.ErroredApplyStatus)

	if numErrored > 0 {
		status = models.FailedCommitStatus
	} else if numSuccess < len(pullStatus.Projects) {
		// If there are plans that haven't been applied yet, we'll use a pending
		// status.
		status = models.PendingCommitStatus
	}

	if err := a.commitStatusUpdater.UpdateCombinedCount(
		ctx.Pull.BaseRepo,
		ctx.Pull,
		status,
		models.ApplyCommand,
		numSuccess,
		len(pullStatus.Projects),
	); err != nil {
		ctx.Log.Warn("unable to update commit status: %s", err)
	}
}

func runProjectCmdsParallel(
	cmds []models.ProjectCommandContext,
	runnerFunc cmdRunnerFunc,
) CommandResult {
	var results []models.ProjectResult
	mux := &sync.Mutex{}

	wg := sizedwaitgroup.New(15)
	for _, pCmd := range cmds {
		pCmd := pCmd
		var execute func()
		wg.Add()

		execute = func() {
			defer wg.Done()
			res := runnerFunc(pCmd)
			mux.Lock()
			results = append(results, res)
			mux.Unlock()
		}

		go execute()
	}

	wg.Wait()
	return CommandResult{ProjectResults: results}
}

func runProjectCmds(
	cmds []models.ProjectCommandContext,
	runnerFunc cmdRunnerFunc,
) CommandResult {
	var results []models.ProjectResult
	for _, pCmd := range cmds {
		res := runnerFunc(pCmd)

		results = append(results, res)
	}
	return CommandResult{ProjectResults: results}
}

// applyAllDisabledComment is posted when apply all commands (i.e. "atlantis apply")
// are disabled and an apply all command is issued.
var applyAllDisabledComment = "**Error:** Running `atlantis apply` without flags is disabled." +
	" You must specify which project to apply via the `-d <dir>`, `-w <workspace>` or `-p <project name>` flags."
