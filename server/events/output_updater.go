package events

import (
	"fmt"

	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
)

// TODO: Move these new structs into it's own package
// server/events/ouptut maybe?

// used to update output for a project
type OutputUpdater interface {
	UpdateOutput(ctx *command.Context, cmd PullCommand, res command.Result)
}

// proxy that forwards req to apprpriate output updater based on VCSHost type
type OutputUpdaterProxy struct {
	outputUpdater map[models.VCSHostType]OutputUpdater
}

func (c *OutputUpdaterProxy) UpdateOutput(ctx *command.Context, cmd PullCommand, res command.Result) {
	c.outputUpdater[ctx.HeadRepo.VCSHost.Type].UpdateOutput(ctx, cmd, res)
}

func NewOutputUpdaterProxy(pullOutputUpdater OutputUpdater, checksOutputUpdater OutputUpdater, enableGithubChecks bool) OutputUpdater {
	// All vcs host have comment output updater configured by default.
	ouputUpdater := map[models.VCSHostType]OutputUpdater{
		models.Github:          pullOutputUpdater,
		models.Gitlab:          pullOutputUpdater,
		models.AzureDevops:     pullOutputUpdater,
		models.BitbucketCloud:  pullOutputUpdater,
		models.BitbucketServer: pullOutputUpdater,
	}

	if enableGithubChecks {
		ouputUpdater[models.Github] = checksOutputUpdater
	}

	return &OutputUpdaterProxy{
		outputUpdater: ouputUpdater,
	}
}

// Used to support checks type output (Github checks for example)
type ChecksOutputUpdater struct {
	VCSClient vcs.Client
}

func (c *ChecksOutputUpdater) UpdateOutput(ctx *command.Context, cmd PullCommand, res command.Result) {
	// TODO: Update status checks output using vcsClient
	// vcsClient.UpdateStatus() which updates the github status check
	// ideally, we would want to checks specific methods but since we're only supporting github for now, this is fine.
	fmt.Printf("DEBUG: %v", res)
}

// Default prj output updater which writes to the pull req comment
type PullOutputUpdater struct {
	HidePrevPlanComments bool
	VCSClient            vcs.Client
	MarkdownRenderer     *MarkdownRenderer
	GlobalCfg            valid.GlobalCfg
}

func (c *PullOutputUpdater) UpdateOutput(ctx *command.Context, cmd PullCommand, res command.Result) {
	// Log if we got any errors or failures.
	if res.Error != nil {
		ctx.Log.Errorf(res.Error.Error())
	} else if res.Failure != "" {
		ctx.Log.Warnf(res.Failure)
	}

	// HidePrevCommandComments will hide old comments left from previous runs to reduce
	// clutter in a pull/merge request. This will not delete the comment, since the
	// comment trail may be useful in auditing or backtracing problems.
	if c.HidePrevPlanComments {
		if err := c.VCSClient.HidePrevCommandComments(ctx.Pull.BaseRepo, ctx.Pull.Num, cmd.CommandName().TitleString()); err != nil {
			ctx.Log.Errorf("unable to hide old comments: %s", err)
		}
	}

	var templateOverrides map[string]string
	repoCfg := c.GlobalCfg.MatchingRepo(ctx.Pull.BaseRepo.ID())
	if repoCfg != nil {
		templateOverrides = repoCfg.TemplateOverrides
	}

	comment := c.MarkdownRenderer.Render(res, cmd.CommandName(), ctx.Pull.BaseRepo.VCSHost.Type, templateOverrides)
	if err := c.VCSClient.CreateComment(ctx.Pull.BaseRepo, ctx.Pull.Num, comment, cmd.CommandName().String()); err != nil {
		ctx.Log.Errorf("unable to comment: %s", err)
	}
}
