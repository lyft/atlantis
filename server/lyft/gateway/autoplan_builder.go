package gateway

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/metrics"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/recovery"
	"github.com/uber-go/tally/v4"
)

// AutoplanValidator handles setting up repo cloning and checking to verify of any terraform files have changed
type AutoplanValidator struct {
	Scope                         tally.Scope
	VCSClient                     vcs.Client
	PreWorkflowHooksCommandRunner events.PreWorkflowHooksCommandRunner
	Drainer                       *events.Drainer
	GlobalCfg                     valid.GlobalCfg
	CommitStatusUpdater           events.CommitStatusUpdater
	PrjCmdBuilder                 events.ProjectPlanCommandBuilder
	OutputUpdater                 events.OutputUpdater
	WorkingDir                    events.WorkingDir
	WorkingDirLocker              events.WorkingDirLocker
}

const DefaultWorkspace = "default"

func (r *AutoplanValidator) isValid(ctx context.Context, logger logging.Logger, baseRepo models.Repo, headRepo models.Repo, pull models.PullRequest, user models.User) (bool, error) {
	if opStarted := r.Drainer.StartOp(); !opStarted {
		return false, errors.New("atlantis is shutting down, cannot process current event")
	}
	defer r.Drainer.OpDone()
	defer r.logPanics(ctx, logger)

	cmdCtx := &command.Context{
		User:       user,
		Log:        logger,
		Scope:      r.Scope,
		Pull:       pull,
		HeadRepo:   headRepo,
		Trigger:    command.AutoTrigger,
		RequestCtx: ctx,
	}
	if !r.validateCtxAndComment(cmdCtx) {
		return false, errors.New("invalid command context")
	}
	err := r.PreWorkflowHooksCommandRunner.RunPreHooks(context.TODO(), cmdCtx)
	if err != nil {
		cmdCtx.Log.ErrorContext(cmdCtx.RequestCtx, fmt.Sprintf("Error running pre-workflow hooks %s. Proceeding with %s command.", err, command.Plan))
	}

	// Set to pending to create checkrun
	if statusErr := r.CommitStatusUpdater.UpdateCombined(context.TODO(), baseRepo, pull, models.PendingCommitStatus, command.Plan); statusErr != nil {
		cmdCtx.Log.WarnContext(cmdCtx.RequestCtx, fmt.Sprintf("unable to update commit status: %v", statusErr))
	}

	projectCmds, err := r.PrjCmdBuilder.BuildAutoplanCommands(cmdCtx)
	if err != nil {
		if statusErr := r.CommitStatusUpdater.UpdateCombined(context.TODO(), baseRepo, pull, models.FailedCommitStatus, command.Plan); statusErr != nil {
			cmdCtx.Log.WarnContext(cmdCtx.RequestCtx, fmt.Sprintf("unable to update commit status: %v", statusErr))
		}
		// If error happened after clone was made, we should clean it up here too
		unlockFn, lockErr := r.WorkingDirLocker.TryLock(baseRepo.FullName, pull.Num, DefaultWorkspace)
		if lockErr != nil {
			cmdCtx.Log.WarnContext(cmdCtx.RequestCtx, "workspace was locked")
			return false, errors.Wrap(err, lockErr.Error())
		}
		defer unlockFn()
		if cloneErr := r.WorkingDir.Delete(baseRepo, pull); cloneErr != nil {
			cmdCtx.Log.WarnContext(cmdCtx.RequestCtx, "unable to delete clone after autoplan failed", map[string]interface{}{"err": cloneErr})
		}
		r.OutputUpdater.UpdateOutput(cmdCtx, events.AutoplanCommand{}, command.Result{Error: err})
		return false, errors.Wrap(err, "Failed building autoplan commands")
	}
	unlockFn, err := r.WorkingDirLocker.TryLock(baseRepo.FullName, pull.Num, DefaultWorkspace)
	if err != nil {
		cmdCtx.Log.WarnContext(cmdCtx.RequestCtx, "workspace was locked")
		return false, err
	}
	defer unlockFn()
	// Delete repo clone generated to validate plan
	if err := r.WorkingDir.Delete(baseRepo, pull); err != nil {
		return false, errors.Wrap(err, "Failed deleting cloned repo")
	}
	if len(projectCmds) == 0 {
		cmdCtx.Log.InfoContext(cmdCtx.RequestCtx, "no modified projects have been found")
		for _, cmd := range []command.Name{command.Plan, command.Apply, command.PolicyCheck} {
			if err := r.CommitStatusUpdater.UpdateCombinedCount(context.TODO(), baseRepo, pull, models.SuccessCommitStatus, cmd, 0, 0); err != nil {
				cmdCtx.Log.WarnContext(cmdCtx.RequestCtx, fmt.Sprintf("unable to update commit status: %s", err))
			}
		}
		return false, nil
	}
	return true, nil
}

func (r *AutoplanValidator) InstrumentedIsValid(ctx context.Context, logger logging.Logger, baseRepo models.Repo, headRepo models.Repo, pull models.PullRequest, user models.User) bool {
	timer := r.Scope.Timer(metrics.ExecutionTimeMetric).Start()
	defer timer.Stop()
	isValid, err := r.isValid(ctx, logger, baseRepo, headRepo, pull, user)

	if err != nil {
		logger.ErrorContext(ctx, err.Error())
		r.Scope.Counter(metrics.ExecutionErrorMetric).Inc(1)
		return false
	}
	if !isValid {
		r.Scope.Counter(metrics.ExecutionFailureMetric).Inc(1)
		return false
	}
	r.Scope.Counter(metrics.ExecutionSuccessMetric).Inc(1)
	return true
}

func (r *AutoplanValidator) logPanics(ctx context.Context, logger logging.Logger) {
	if err := recover(); err != nil {
		stack := recovery.Stack(3)
		logger.ErrorContext(ctx, fmt.Sprintf("PANIC: %s\n%s", err, stack))
	}
}

func (r *AutoplanValidator) validateCtxAndComment(cmdCtx *command.Context) bool {
	if cmdCtx.HeadRepo.Owner != cmdCtx.Pull.BaseRepo.Owner {
		cmdCtx.Log.InfoContext(cmdCtx.RequestCtx, "command was run on a fork pull request which is disallowed")
		return false
	}

	if cmdCtx.Pull.State != models.OpenPullState {
		cmdCtx.Log.InfoContext(cmdCtx.RequestCtx, "command was run on closed pull request")
		return false
	}

	repo := r.GlobalCfg.MatchingRepo(cmdCtx.Pull.BaseRepo.ID())
	if !repo.BranchMatches(cmdCtx.Pull.BaseBranch) {
		cmdCtx.Log.InfoContext(cmdCtx.RequestCtx, "command was run on a pull request which doesn't match base branches")
		// just ignore it to allow us to use any git workflows without malicious intentions.
		return false
	}
	return true
}
