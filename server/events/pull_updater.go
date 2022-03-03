package events

import (
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/lyft/feature"
)

type PullUpdater interface {
	updatePull(ctx *command.Context, cmd PullCommand, res command.Result)
}
type DefaultPullUpdater struct {
	PullUpdater
	HidePrevPlanComments bool
	VCSClient            vcs.Client
	MarkdownRenderer     *MarkdownRenderer
	GlobalCfg            valid.GlobalCfg
}

func (c *DefaultPullUpdater) updatePull(ctx *command.Context, cmd PullCommand, res command.Result) {
	// Log if we got any errors or failures.
	if res.Error != nil {
		ctx.Log.Err(res.Error.Error())
	} else if res.Failure != "" {
		ctx.Log.Warn(res.Failure)
	}

	// HidePrevCommandComments will hide old comments left from previous runs to reduce
	// clutter in a pull/merge request. This will not delete the comment, since the
	// comment trail may be useful in auditing or backtracing problems.
	if c.HidePrevPlanComments {
		if err := c.VCSClient.HidePrevCommandComments(ctx.Pull.BaseRepo, ctx.Pull.Num, cmd.CommandName().TitleString()); err != nil {
			ctx.Log.Err("unable to hide old comments: %s", err)
		}
	}

	var templateOverrides map[string]string
	repoCfg := c.GlobalCfg.MatchingRepo(ctx.Pull.BaseRepo.ID())
	if repoCfg != nil {
		templateOverrides = repoCfg.TemplateOverrides
	}

	comment := c.MarkdownRenderer.Render(res, cmd.CommandName(), ctx.Log.GetHistory(), cmd.IsVerbose(), ctx.Pull.BaseRepo.VCSHost.Type, templateOverrides)
	if err := c.VCSClient.CreateComment(ctx.Pull.BaseRepo, ctx.Pull.Num, comment, cmd.CommandName().String()); err != nil {
		ctx.Log.Err("unable to comment: %s", err)
	}
}

type FeatureAwarePullUpdater struct {
	PullUpdater
	HidePrevPlanComments bool
	VCSClient            vcs.Client
	MarkdownRenderer     *MarkdownRenderer
	GlobalCfg            valid.GlobalCfg
	FeatureAllocator     feature.Allocator
}

func (f *FeatureAwarePullUpdater) updatePull(ctx *command.Context, cmd PullCommand, res command.Result) {
	githubChecks, err := f.FeatureAllocator.ShouldAllocate(feature.GitHubChecks, ctx.HeadRepo.FullName)
	if err != nil {
		githubChecks = false
	}
	if !githubChecks {
		f.PullUpdater.updatePull(ctx, cmd, res)
		return
	}

	// Log if we got any errors or failures.
	if res.Error != nil {
		ctx.Log.Err(res.Error.Error())
	} else if res.Failure != "" {
		ctx.Log.Warn(res.Failure)
	}

	var templateOverrides map[string]string
	repoCfg := f.GlobalCfg.MatchingRepo(ctx.Pull.BaseRepo.ID())
	if repoCfg != nil {
		templateOverrides = repoCfg.TemplateOverrides
	}
	// For github checks we want to render the markdown for each project separately.
	for _, projectResult := range res.ProjectResults {
		comment := f.MarkdownRenderer.RenderSingleProjectResult(projectResult, cmd.CommandName(), ctx.Log.GetHistory(), false, ctx.Pull.BaseRepo.VCSHost.Type, templateOverrides)

		var commitStatus models.CommitStatus
		if projectResult.Error != nil && projectResult.Failure != "" {
			commitStatus = models.FailedCommitStatus
		} else {
			commitStatus = models.SuccessCommitStatus
		}
		if err := f.VCSClient.UpdateCheckRun(ctx.Pull.BaseRepo, ctx.Pull, projectResult.CheckID, commitStatus, cmd.CommandName(), "", comment); err != nil {
			ctx.Log.Err("unable to update check: %s", err)
		}
	}
}
