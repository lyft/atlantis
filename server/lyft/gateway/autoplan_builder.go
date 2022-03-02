package gateway

import (
	"fmt"
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

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_autoplan_validator.go AutoplanValidator
type AutoplanValidator interface {
	PullRequestHasTerraformChanges(baseRepo models.Repo, headRepo models.Repo, pull models.PullRequest, user models.User) bool
}

// AutoplanBuilder handles setting up repo cloning and checking to verify of any terraform files have changed
type AutoplanBuilder struct {
	Logger                        logging.SimpleLogging
	Scope                         tally.Scope
	VCSClient                     vcs.Client
	PreWorkflowHooksCommandRunner events.PreWorkflowHooksCommandRunner
	DisableAutoplan               bool
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
	silenceVCSStatusNoPlans bool
	// SilenceVCSStatusNoPlans is whether any plan should set commit status if no projects
	// are found
	silenceVCSStatusNoProjects bool
	CommitStatusUpdater        events.CommitStatusUpdater
	PrjCmdBuilder              events.ProjectPlanCommandBuilder
	PullUpdater                *events.PullUpdater
}

func (r *AutoplanBuilder) PullRequestHasTerraformChanges(baseRepo models.Repo, headRepo models.Repo, pull models.PullRequest, user models.User) bool {
	if opStarted := r.Drainer.StartOp(); !opStarted {
		if commentErr := r.VCSClient.CreateComment(baseRepo, pull.Num, ShutdownComment, command.Plan.String()); commentErr != nil {
			r.Logger.Log(logging.Error, "unable to comment that Atlantis is shutting down: %s", commentErr)
		}
		return false
	}
	defer r.Drainer.OpDone()

	log := r.Logger.WithHistory(
		"repository", baseRepo.FullName,
		"pull-num", strconv.Itoa(pull.Num),
	)
	defer r.logPanics(baseRepo, pull.Num, log)

	scope := r.Scope.SubScope("gateway-autoplan")
	timer := scope.Timer(metrics.ExecutionTimeMetric).Start()
	defer timer.Stop()

	ctx := &command.Context{
		User:     user,
		Log:      log,
		Scope:    scope,
		Pull:     pull,
		HeadRepo: headRepo,
		Trigger:  command.AutoTrigger,
	}
	if !r.validateCtxAndComment(ctx) {
		return false
	}
	if r.DisableAutoplan {
		return false
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
		r.PullUpdater.UpdatePull(ctx, events.AutoplanCommand{}, command.Result{Error: err})
		return false
	}
	if len(projectCmds) == 0 {
		ctx.Log.Info("determined there was no project to run plan in")
		if !(r.silenceVCSStatusNoPlans || r.silenceVCSStatusNoProjects) {
			// If there were no projects modified, we set successful commit statuses
			// with 0/0 projects planned/policy_checked/applied successfully because some users require
			// the Atlantis status to be passing for all pull requests.
			ctx.Log.Debug("setting VCS status to success with no projects found")
			if err := r.CommitStatusUpdater.UpdateCombinedCount(baseRepo, pull, models.SuccessCommitStatus, command.Plan, 0, 0); err != nil {
				ctx.Log.Warn("unable to update commit status: %s", err)
			}
			if err := r.CommitStatusUpdater.UpdateCombinedCount(baseRepo, pull, models.SuccessCommitStatus, command.PolicyCheck, 0, 0); err != nil {
				ctx.Log.Warn("unable to update commit status: %s", err)
			}
			if err := r.CommitStatusUpdater.UpdateCombinedCount(baseRepo, pull, models.SuccessCommitStatus, command.Apply, 0, 0); err != nil {
				ctx.Log.Warn("unable to update commit status: %s", err)
			}
		}
		ctx.Scope.Counter("tf_projects_found").Inc(1)
		return false
	}
	ctx.Scope.Counter("tf_projects_not_found").Inc(1)
	return true
}

func (r *AutoplanBuilder) logPanics(baseRepo models.Repo, pullNum int, logger logging.SimpleLogging) {
	if err := recover(); err != nil {
		stack := recovery.Stack(3)
		logger.Err("PANIC: %s\n%s", err, stack)
		if commentErr := r.VCSClient.CreateComment(
			baseRepo,
			pullNum,
			fmt.Sprintf("**Error: goroutine panic. This is a bug.**\n```\n%s\n%s```", err, stack),
			"",
		); commentErr != nil {
			logger.Err("unable to comment: %s", commentErr)
		}
	}
}

func (r *AutoplanBuilder) validateCtxAndComment(ctx *command.Context) bool {
	if !r.AllowForkPRs && ctx.HeadRepo.Owner != ctx.Pull.BaseRepo.Owner {
		if r.SilenceForkPRErrors {
			return false
		}
		ctx.Log.Info("command was run on a fork pull request which is disallowed")
		if err := r.VCSClient.CreateComment(ctx.Pull.BaseRepo, ctx.Pull.Num, fmt.Sprintf("Atlantis commands can't be run on fork pull requests. To enable, set --%s  or, to disable this message, set --%s", r.AllowForkPRsFlag, r.SilenceForkPRErrorsFlag), ""); err != nil {
			ctx.Log.Err("unable to comment: %s", err)
		}
		return false
	}

	if ctx.Pull.State != models.OpenPullState {
		ctx.Log.Info("command was run on closed pull request")
		if err := r.VCSClient.CreateComment(ctx.Pull.BaseRepo, ctx.Pull.Num, "Atlantis commands can't be run on closed pull requests", ""); err != nil {
			ctx.Log.Err("unable to comment: %s", err)
		}
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
