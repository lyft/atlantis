package gateway

import (
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/metrics"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/recovery"
	"github.com/uber-go/tally"
	"strconv"
)

// AutoplanValidator handles setting up repo cloning and checking to verify of any terraform files have changed
type AutoplanValidator struct {
	Logger                        logging.SimpleLogging
	Scope                         tally.Scope
	VCSClient                     vcs.Client
	PreWorkflowHooksCommandRunner events.PreWorkflowHooksCommandRunner
	Drainer                       *events.Drainer
	GlobalCfg                     valid.GlobalCfg
	// AllowForkPRs controls whether we operate on pull requests from forks.
	AllowForkPRs bool
	// AllowForkPRsFlag is the name of the flag that controls fork PR's. We use
	// this in our error message back to the user on a forked PR so they know
	// how to enable this functionality.
	AllowForkPRsFlag string
	// SilenceForkPRErrors controls whether to comment on Fork PRs when AllowForkPRs = False
	SilenceForkPRErrors bool
	// SilenceForkPRErrorsFlag is the name of the flag that controls fork PR's. We use
	// this in our error message back to the user on a forked PR so they know
	// how to disable error comment
	SilenceForkPRErrorsFlag string
	// SilenceVCSStatusNoPlans is whether autoplan should set commit status if no plans
	// are found
	SilenceVCSStatusNoPlans bool
	// SilenceVCSStatusNoPlans is whether any plan should set commit status if no projects
	// are found
	SilenceVCSStatusNoProjects bool
	CommitStatusUpdater        events.CommitStatusUpdater
	PrjCmdBuilder              events.ProjectPlanCommandBuilder
	PullUpdater                *events.PullUpdater
	WorkingDir                 events.WorkingDir
	WorkingDirLocker           events.WorkingDirLocker
}

const DefaultWorkspace = "default"

func (r *AutoplanValidator) isValid(baseRepo models.Repo, headRepo models.Repo, pull models.PullRequest, user models.User) (bool, error) {
	if opStarted := r.Drainer.StartOp(); !opStarted {
		return false, errors.New("atlantis is shutting down, cannot process current event")
	}
	defer r.Drainer.OpDone()

	log := r.Logger.WithHistory(
		"repository", baseRepo.FullName,
		"pull-num", strconv.Itoa(pull.Num),
	)
	defer r.logPanics(log)

	ctx := &command.Context{
		User:     user,
		Log:      log,
		Scope:    r.Scope,
		Pull:     pull,
		HeadRepo: headRepo,
		Trigger:  command.AutoTrigger,
	}
	if !r.validateCtxAndComment(ctx) {
		return false, errors.New("invalid command context")
	}
	err := r.PreWorkflowHooksCommandRunner.RunPreHooks(ctx)
	if err != nil {
		ctx.Log.Err("Error running pre-workflow hooks %s. Proceeding with %s command.", err, command.Plan)
	}

	projectCmds, err := r.PrjCmdBuilder.BuildAutoplanCommands(ctx)
	if err != nil {
		if statusErr := r.CommitStatusUpdater.UpdateCombined(baseRepo, pull, models.FailedCommitStatus, command.Plan); statusErr != nil {
			ctx.Log.Warn("unable to update commit status: %s", statusErr)
		}
		// If error happened after clone was made, we should clean it up here too
		unlockFn, lockErr := r.WorkingDirLocker.TryLock(baseRepo.FullName, pull.Num, DefaultWorkspace)
		if lockErr != nil {
			ctx.Log.Warn("workspace was locked")
			return false, errors.Wrap(err, lockErr.Error())
		}
		defer unlockFn()
		if cloneErr := r.WorkingDir.Delete(baseRepo, pull); cloneErr != nil {
			ctx.Log.With("err", cloneErr).Warn("unable to delete clone after autoplan failed")
		}
		r.PullUpdater.UpdatePull(ctx, events.AutoplanCommand{}, command.Result{Error: err})
		return false, errors.Wrap(err, "Failed building autoplan commands")
	}
	unlockFn, err := r.WorkingDirLocker.TryLock(baseRepo.FullName, pull.Num, DefaultWorkspace)
	if err != nil {
		ctx.Log.Warn("workspace was locked")
		return false, err
	}
	defer unlockFn()
	// Delete repo clone generated to validate plan
	if err := r.WorkingDir.Delete(baseRepo, pull); err != nil {
		return false, errors.Wrap(err, "Failed deleting cloned repo")
	}
	if len(projectCmds) == 0 {
		ctx.Log.Info("determined there was no project to run plan in")
		if !(r.SilenceVCSStatusNoPlans || r.SilenceVCSStatusNoProjects) {
			// If there were no projects modified, we set successful commit statuses
			// with 0/0 projects planned/policy_checked/applied successfully because some users require
			// the Atlantis status to be passing for all pull requests.
			ctx.Log.Debug("setting VCS status to success with no projects found")
			for _, cmd := range []command.Name{command.Plan, command.Apply, command.PolicyCheck} {
				if err := r.CommitStatusUpdater.UpdateCombinedCount(baseRepo, pull, models.SuccessCommitStatus, cmd, 0, 0); err != nil {
					ctx.Log.Warn("unable to update commit status: %s", err)
				}
			}
		}
		return false, nil
	}
	return true, nil
}

func (r *AutoplanValidator) InstrumentedIsValid(baseRepo models.Repo, headRepo models.Repo, pull models.PullRequest, user models.User) bool {
	timer := r.Scope.Timer(metrics.ExecutionTimeMetric).Start()
	defer timer.Stop()
	isValid, err := r.isValid(baseRepo, headRepo, pull, user)
	if err != nil {
		r.Logger.With("repo", baseRepo.FullName, "pull", pull.Num).Err(err.Error())
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

func (r *AutoplanValidator) logPanics(logger logging.SimpleLogging) {
	if err := recover(); err != nil {
		stack := recovery.Stack(3)
		logger.Err("PANIC: %s\n%s", err, stack)
	}
}

func (r *AutoplanValidator) validateCtxAndComment(ctx *command.Context) bool {
	if !r.AllowForkPRs && ctx.HeadRepo.Owner != ctx.Pull.BaseRepo.Owner {
		if r.SilenceForkPRErrors {
			return false
		}
		ctx.Log.Info("command was run on a fork pull request which is disallowed")
		return false
	}

	if ctx.Pull.State != models.OpenPullState {
		ctx.Log.Info("command was run on closed pull request")
		return false
	}

	repo := r.GlobalCfg.MatchingRepo(ctx.Pull.BaseRepo.ID())
	if !repo.BranchMatches(ctx.Pull.BaseBranch) {
		ctx.Log.Info("command was run on a pull request which doesn't match base branches")
		// just ignore it to allow us to use any git workflows without malicious intentions.
		return false
	}
	return true
}
