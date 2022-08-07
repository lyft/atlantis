package events

import (
	"fmt"
	"strings"

	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/events/vcs/types"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/lyft/feature"
	"github.com/runatlantis/atlantis/server/vcs/markdown"
)

type OutputUpdater interface {
	UpdateOutput(ctx *command.Context, cmd PullCommand, res command.Result)
}

// [WENGINES-4643] TODO: Remove PullOutputUpdater and default to checks once github checks is stable
// defaults to pull comments if checks is turned off
type FeatureAwareChecksOutputUpdater struct {
	PullOutputUpdater
	ChecksOutputUpdater

	FeatureAllocator feature.Allocator
	Logger           logging.Logger
}

func (c *FeatureAwareChecksOutputUpdater) UpdateOutput(ctx *command.Context, cmd PullCommand, res command.Result) {
	shouldAllocate, err := c.FeatureAllocator.ShouldAllocate(feature.GithubChecks, feature.FeatureContext{
		RepoName:         ctx.HeadRepo.FullName,
		PullCreationTime: ctx.Pull.CreatedAt,
	})
	if err != nil {
		c.Logger.ErrorContext(ctx.RequestCtx, fmt.Sprintf("unable to allocate for feature: %s, error: %s", feature.GithubChecks, err))
	}

	// Github Checks turned on and github provider
	if ctx.HeadRepo.VCSHost.Type == models.Github && shouldAllocate {
		c.ChecksOutputUpdater.UpdateOutput(ctx, cmd, res)
		return
	}
	c.PullOutputUpdater.UpdateOutput(ctx, cmd, res)
}

// Used to support checks type output (Github checks for example)
type ChecksOutputUpdater struct {
	VCSClient        vcs.Client
	MarkdownRenderer *markdown.Renderer
	TitleBuilder     vcs.StatusTitleBuilder
}

func (c *ChecksOutputUpdater) handlePolicyCheck(ctx command.Context, cmd PullCommand, res command.Result) {
	// Write output to description
	for _, projectResult := range res.ProjectResults {
		statusName := c.TitleBuilder.Build(cmd.CommandName().String(), vcs.StatusTitleOptions{
			ProjectName: projectResult.ProjectName,
		})

		var state models.CommitStatus
		if projectResult.Error != nil || projectResult.Failure != "" {
			state = models.FailedCommitStatus
		} else {
			state = models.SuccessCommitStatus
		}

		updateStatusReq := types.UpdateStatusRequest{
			Repo:        ctx.HeadRepo,
			Ref:         ctx.Pull.HeadCommit,
			StatusName:  statusName,
			PullNum:     ctx.Pull.Num,
			State:       state,
			Description: c.MarkdownRenderer.RenderProject(projectResult, cmd.CommandName(), ctx.Pull.BaseRepo),
		}

		if err := c.VCSClient.UpdateStatus(ctx.RequestCtx, updateStatusReq); err != nil {
			ctx.Log.ErrorContext(ctx.RequestCtx, "updable to update check run", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}
}

func (c *ChecksOutputUpdater) UpdateOutput(ctx *command.Context, cmd PullCommand, res command.Result) {

	if res.Error != nil || res.Failure != "" {
		output := c.MarkdownRenderer.Render(res, cmd.CommandName(), ctx.Pull.BaseRepo)
		updateStatusReq := types.UpdateStatusRequest{
			Repo:        ctx.HeadRepo,
			Ref:         ctx.Pull.HeadCommit,
			StatusName:  c.TitleBuilder.Build(cmd.CommandName().String()),
			PullNum:     ctx.Pull.Num,
			Description: fmt.Sprintf("%s failed", strings.Title(cmd.CommandName().String())),
			Output:      output,
			State:       models.FailedCommitStatus,
		}

		if err := c.VCSClient.UpdateStatus(ctx.RequestCtx, updateStatusReq); err != nil {
			ctx.Log.ErrorContext(ctx.RequestCtx, "updable to update check run", map[string]interface{}{
				"error": err.Error(),
			})
		}
		return
	}

	// Write output to description
	if cmd.CommandName() == command.PolicyCheck {
		c.handlePolicyCheck(*ctx, cmd, res)
		return
	}

	// Handle ApprovePolicies command separately
	if cmd.CommandName() == command.ApprovePolicies {
		c.handleApprovePolicies(ctx, cmd, res)
		return
	}

	// iterate through all project results and the update the github check
	for _, projectResult := range res.ProjectResults {
		statusName := c.TitleBuilder.Build(cmd.CommandName().String(), vcs.StatusTitleOptions{
			ProjectName: projectResult.ProjectName,
		})

		// Description is a required field
		description := fmt.Sprintf("**Project**: `%s`\n**Dir**: `%s`\n**Workspace**: `%s`", projectResult.ProjectName, projectResult.RepoRelDir, projectResult.Workspace)

		var state models.CommitStatus
		if projectResult.Error != nil || projectResult.Failure != "" {
			state = models.FailedCommitStatus
		} else {
			state = models.SuccessCommitStatus
		}

		output := c.MarkdownRenderer.RenderProject(projectResult, cmd.CommandName(), ctx.Pull.BaseRepo)
		updateStatusReq := types.UpdateStatusRequest{
			Repo:        ctx.HeadRepo,
			Ref:         ctx.Pull.HeadCommit,
			StatusName:  statusName,
			PullNum:     ctx.Pull.Num,
			Description: description,
			Output:      output,
			State:       state,
		}

		if err := c.VCSClient.UpdateStatus(ctx.RequestCtx, updateStatusReq); err != nil {
			ctx.Log.ErrorContext(ctx.RequestCtx, "unable to update check run", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

}

func (c *ChecksOutputUpdater) handleApprovePolicies(ctx *command.Context, cmd PullCommand, res command.Result) {
	output := c.MarkdownRenderer.Render(res, cmd.CommandName(), ctx.Pull.BaseRepo)
	updateStatusReq := types.UpdateStatusRequest{
		Repo:        ctx.HeadRepo,
		Ref:         ctx.Pull.HeadCommit,
		StatusName:  c.TitleBuilder.Build(cmd.CommandName().String()),
		PullNum:     ctx.Pull.Num,
		Description: fmt.Sprintf("%s succeded", strings.Title(cmd.CommandName().String())),
		Output:      output,
		State:       models.SuccessCommitStatus,
	}

	if err := c.VCSClient.UpdateStatus(ctx.RequestCtx, updateStatusReq); err != nil {
		ctx.Log.Error("unable to update check run", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// In addition, update project level atlantis/policy_check checkruns
	for _, projectResult := range res.ProjectResults {
		statusName := c.TitleBuilder.Build(command.PolicyCheck.String(), vcs.StatusTitleOptions{
			ProjectName: projectResult.ProjectName,
		})

		var state models.CommitStatus
		if projectResult.Error != nil || projectResult.Failure != "" {
			state = models.FailedCommitStatus
		} else {
			state = models.SuccessCommitStatus
		}

		updateStatusReq := types.UpdateStatusRequest{
			Repo:       ctx.HeadRepo,
			Ref:        ctx.Pull.HeadCommit,
			StatusName: statusName,
			PullNum:    ctx.Pull.Num,
			State:      state,
		}

		if err := c.VCSClient.UpdateStatus(ctx.RequestCtx, updateStatusReq); err != nil {
			ctx.Log.ErrorContext(ctx.RequestCtx, "updable to update check run", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	return
}

// Default prj output updater which writes to the pull req comment
type PullOutputUpdater struct {
	HidePrevPlanComments bool
	VCSClient            vcs.Client
	MarkdownRenderer     *markdown.Renderer
}

func (c *PullOutputUpdater) UpdateOutput(ctx *command.Context, cmd PullCommand, res command.Result) {
	// Log if we got any errors or failures.
	if res.Error != nil {
		ctx.Log.ErrorContext(ctx.RequestCtx, "", map[string]interface{}{
			"error": res.Error.Error(),
		})
	} else if res.Failure != "" {
		ctx.Log.WarnContext(ctx.RequestCtx, "", map[string]interface{}{
			"failiure": res.Failure,
		})
	}

	// HidePrevCommandComments will hide old comments left from previous runs to reduce
	// clutter in a pull/merge request. This will not delete the comment, since the
	// comment trail may be useful in auditing or backtracing problems.
	if c.HidePrevPlanComments {
		if err := c.VCSClient.HidePrevCommandComments(ctx.Pull.BaseRepo, ctx.Pull.Num, cmd.CommandName().TitleString()); err != nil {
			ctx.Log.ErrorContext(ctx.RequestCtx, "unable to hide old comments", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	comment := c.MarkdownRenderer.Render(res, cmd.CommandName(), ctx.Pull.BaseRepo)
	if err := c.VCSClient.CreateComment(ctx.Pull.BaseRepo, ctx.Pull.Num, comment, cmd.CommandName().String()); err != nil {
		ctx.Log.ErrorContext(ctx.RequestCtx, "unable to comment", map[string]interface{}{
			"error": err.Error(),
		})
	}
}
