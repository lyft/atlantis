package events

import (
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
)

type PullUpdater struct {
	HidePrevPlanComments bool
	VCSClient            vcs.Client
	MarkdownRenderer     *MarkdownRenderer
	GlobalCfg            valid.GlobalCfg
}

func (c *PullUpdater) updatePull(ctx *command.Context, cmd PullCommand, res command.Result) {
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

func (c *PullUpdater) createCheckRun(ctx *command.Context, status models.CommitStatus, commandName command.Name, res command.Result) int64 {

	// Log if we got any errors or failures.
	if res.Error != nil {
		ctx.Log.Err(res.Error.Error())
	} else if res.Failure != "" {
		ctx.Log.Warn(res.Failure)
	}

	var templateOverrides map[string]string
	repoCfg := c.GlobalCfg.MatchingRepo(ctx.Pull.BaseRepo.ID())
	if repoCfg != nil {
		templateOverrides = repoCfg.TemplateOverrides
	}

	comment := c.MarkdownRenderer.Render(res, commandName, ctx.Log.GetHistory(), false, ctx.Pull.BaseRepo.VCSHost.Type, templateOverrides)

	checkID, err := c.VCSClient.CreateCheckRun(ctx.Pull.BaseRepo, ctx.Pull, status, commandName, comment)
	if err != nil {
		ctx.Log.Err("unable to create check: %s", err)
	}
	return checkID
}

func (c *PullUpdater) updateCheckRun(ctx *command.Context, checkId int64, status models.CommitStatus, commandName command.Name, res command.Result) {
	// Log if we got any errors or failures.
	if res.Error != nil {
		ctx.Log.Err(res.Error.Error())
	} else if res.Failure != "" {
		ctx.Log.Warn(res.Failure)
	}

	var templateOverrides map[string]string
	repoCfg := c.GlobalCfg.MatchingRepo(ctx.Pull.BaseRepo.ID())
	if repoCfg != nil {
		templateOverrides = repoCfg.TemplateOverrides
	}
	comment := c.MarkdownRenderer.Render(res, commandName, ctx.Log.GetHistory(), false, ctx.Pull.BaseRepo.VCSHost.Type, templateOverrides)

	if err := c.VCSClient.UpdateCheckRun(ctx.Pull.BaseRepo, ctx.Pull, checkId, status, commandName, comment); err != nil {
		ctx.Log.Err("unable to update check: %s", err)
	}
}
