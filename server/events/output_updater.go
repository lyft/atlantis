package events

import (
	"context"
	"fmt"

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

type renderer interface {
	Render(res command.Result, cmdName command.Name, baseRepo models.Repo) string
	RenderProject(prjRes command.ProjectResult, cmdName command.Name, baseRepo models.Repo) string
}

type checksClient interface {
	UpdateStatus(ctx context.Context, request types.UpdateStatusRequest) (string, error)
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
	VCSClient        checksClient
	MarkdownRenderer renderer
	TitleBuilder     vcs.StatusTitleBuilder
}

func (c *ChecksOutputUpdater) UpdateOutput(ctx *command.Context, cmd PullCommand, res command.Result) {

	// iterate through all project results and the update the github check
	for _, projectResult := range res.ProjectResults {
		updateStatusReq := types.UpdateStatusRequest{
			Repo:             ctx.HeadRepo,
			Ref:              ctx.Pull.HeadCommit,
			PullNum:          ctx.Pull.Num,
			PullCreationTime: ctx.Pull.CreatedAt,
			StatusId:         projectResult.StatusId,
			StatusName:       c.buildStatusName(cmd, projectResult),
			Description:      c.buildDescription(projectResult),
			Output:           c.MarkdownRenderer.RenderProject(projectResult, projectResult.Command, ctx.HeadRepo),
			State:            c.resolveState(projectResult),
		}

		if _, err := c.VCSClient.UpdateStatus(ctx.RequestCtx, updateStatusReq); err != nil {
			ctx.Log.ErrorContext(ctx.RequestCtx, "unable to update check run", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}
}

// Replace project level approve policies command with Policy Check
func (c *ChecksOutputUpdater) buildStatusName(cmd PullCommand, prjResult command.ProjectResult) string {
	commandName := cmd.CommandName()
	if commandName == command.ApprovePolicies {
		commandName = command.PolicyCheck
	}

	return c.TitleBuilder.Build(commandName.String(), vcs.StatusTitleOptions{
		ProjectName: prjResult.ProjectName,
	})
}

func (c *ChecksOutputUpdater) buildDescription(projectResult command.ProjectResult) string {
	return fmt.Sprintf("**Project**: `%s`\n**Dir**: `%s`\n**Workspace**: `%s`", projectResult.ProjectName, projectResult.RepoRelDir, projectResult.Workspace)
}

func (c *ChecksOutputUpdater) resolveState(result command.ProjectResult) models.CommitStatus {
	if result.Error != nil || result.Failure != "" {
		return models.FailedCommitStatus
	} else {
		return models.SuccessCommitStatus
	}
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
