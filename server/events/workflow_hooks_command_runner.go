package events

import (
	"fmt"

	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/runtime"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/events/yaml/valid"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/recovery"
)

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_workflows_hooks_command_runner.go WorkflowHooksCommandRunner

type WorkflowHooksCommandRunner interface {
	RunPreHooks(
		baseRepo models.Repo,
		headRepo models.Repo,
		pull models.PullRequest,
		user models.User,
	)
}

// DefaultWorkflowHooksCommandRunner is the first step when processing a workflow hook commands.
type DefaultWorkflowHooksCommandRunner struct {
	VCSClient          vcs.Client
	Logger             logging.SimpleLogging
	WorkingDirLocker   WorkingDirLocker
	WorkingDir         WorkingDir
	GlobalCfg          valid.GlobalCfg
	Drainer            *Drainer
	WorkflowHookRunner *runtime.WorkflowHookRunner
}

// RunPreHooks runs pre_workflow_hooks when PR is opened or updated.
func (w *DefaultWorkflowHooksCommandRunner) RunPreHooks(
	baseRepo models.Repo,
	headRepo models.Repo,
	pull models.PullRequest,
	user models.User,
) {
	if opStarted := w.Drainer.StartOp(); !opStarted {
		if commentErr := w.VCSClient.CreateComment(baseRepo, pull.Num, ShutdownComment, "pre_workflow_hooks"); commentErr != nil {
			w.Logger.Log(logging.Error, "unable to comment that Atlantis is shutting down: %s", commentErr)
		}
		return
	}
	defer w.Drainer.OpDone()

	log := w.buildLogger(baseRepo.FullName, pull.Num)
	defer w.logPanics(baseRepo, pull.Num, log)

	log.Info("Running Pre Hooks for repo: ")

	unlockFn, err := w.WorkingDirLocker.TryLock(baseRepo.FullName, pull.Num, DefaultWorkspace)
	if err != nil {
		log.Warn("workspace is locked")
		return
	}
	log.Debug("got workspace lock")
	defer unlockFn()

	repoDir, _, err := w.WorkingDir.Clone(log, headRepo, pull, DefaultWorkspace)
	if err != nil {
		log.Err("unable to run pre workflow hooks: %s", err)
		return
	}

	workflowHooks := make([]*valid.WorkflowHook, 0)
	for _, repo := range w.GlobalCfg.Repos {
		if repo.IDMatches(baseRepo.ID()) && len(repo.WorkflowHooks) > 0 {
			workflowHooks = append(workflowHooks, repo.WorkflowHooks...)
		}
	}

	ctx := models.WorkflowHookCommandContext{
		BaseRepo: baseRepo,
		HeadRepo: headRepo,
		Log:      log,
		Pull:     pull,
		User:     user,
		Verbose:  false,
	}

	result := w.runHooks(ctx, workflowHooks, repoDir)

	if result.HasErrors() {
		log.Err("pre workflow hook run error results: %s", result.Errors())
	}
	return

}

func (w *DefaultWorkflowHooksCommandRunner) runHooks(
	ctx models.WorkflowHookCommandContext,
	workflowHooks []*valid.WorkflowHook,
	repoDir string,
) *WorkflowHooksCommandResult {
	result := &WorkflowHooksCommandResult{
		WorkflowHookResults: make([]models.WorkflowHookResult, 0),
	}
	for _, hook := range workflowHooks {
		out, err := w.WorkflowHookRunner.Run(ctx, hook.RunCommand, repoDir)

		res := models.WorkflowHookResult{
			Output: out,
		}

		if err != nil {
			res.Error = err
			res.Success = false
		} else {
			res.Success = true
		}

		result.WorkflowHookResults = append(result.WorkflowHookResults, res)

		if !res.IsSuccessful() {
			return result
		}
	}

	return result
}

func (w *DefaultWorkflowHooksCommandRunner) buildLogger(repoFullName string, pullNum int) *logging.SimpleLogger {
	src := fmt.Sprintf("%s#%d", repoFullName, pullNum)
	return w.Logger.NewLogger(src, true, w.Logger.GetLevel())
}

// logPanics logs and creates a comment on the pull request for panics.
func (w *DefaultWorkflowHooksCommandRunner) logPanics(baseRepo models.Repo, pullNum int, logger logging.SimpleLogging) {
	if err := recover(); err != nil {
		stack := recovery.Stack(3)
		logger.Err("PANIC: %s\n%s", err, stack)
		if commentErr := w.VCSClient.CreateComment(
			baseRepo,
			pullNum,
			fmt.Sprintf("**Error: goroutine panic. This is a bug.**\n```\n%s\n%s```", err, stack),
			"",
		); commentErr != nil {
			logger.Err("unable to comment: %s", commentErr)
		}
	}
}
