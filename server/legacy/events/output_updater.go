package events

import (
	"context"
	"fmt"

	"github.com/runatlantis/atlantis/server/legacy/events/command"
	"github.com/runatlantis/atlantis/server/legacy/events/vcs"
	"github.com/runatlantis/atlantis/server/legacy/events/vcs/types"
	"github.com/runatlantis/atlantis/server/models"
	"github.com/runatlantis/atlantis/server/vcs/markdown"
)

type OutputUpdater interface {
	UpdateOutput(ctx *command.Context, cmd PullCommand, res command.Result)
}

// JobURLGenerator generates urls to view project's progress.
type jobURLGenerator interface {
	GenerateProjectJobURL(jobID string) (string, error)
}

type renderer interface {
	Render(res command.Result, cmdName command.Name, baseRepo models.Repo) string
	RenderProject(prjRes command.ProjectResult, cmdName fmt.Stringer, baseRepo models.Repo) string
}

type checksClient interface {
	UpdateStatus(ctx context.Context, request types.UpdateStatusRequest) (string, error)
}

// Used to support checks type output (Github checks for example)
type ChecksOutputUpdater struct {
	VCSClient        checksClient
	MarkdownRenderer renderer
	TitleBuilder     vcs.StatusTitleBuilder
	JobURLGenerator  jobURLGenerator
}

func (c *ChecksOutputUpdater) UpdateOutput(ctx *command.Context, cmd PullCommand, res command.Result) {
	if res.Error != nil || res.Failure != "" {
		c.handleCommandFailure(ctx, cmd, res)
		return
	}

	// iterate through all project results and the update the github check
	for _, projectResult := range res.ProjectResults {
		updateStatusReq := types.UpdateStatusRequest{
			Repo:             ctx.HeadRepo,
			Ref:              ctx.Pull.HeadCommit,
			PullNum:          ctx.Pull.Num,
			PullCreationTime: ctx.Pull.CreatedAt,
			StatusID:         projectResult.StatusID,
			DetailsURL:       c.buildJobURL(ctx, projectResult.Command, projectResult.JobID),
			Output:           c.MarkdownRenderer.RenderProject(projectResult, projectResult.Command, ctx.HeadRepo),
			State:            c.resolveState(projectResult),

			StatusName: c.buildStatusName(cmd, vcs.StatusTitleOptions{ProjectName: projectResult.ProjectName}),

			// Additional fields to support templating for project level checkruns
			CommandName: projectResult.Command.TitleString(),
			Project:     projectResult.ProjectName,
			Workspace:   projectResult.Workspace,
			Directory:   projectResult.RepoRelDir,
		}

		if _, err := c.VCSClient.UpdateStatus(ctx.RequestCtx, updateStatusReq); err != nil {
			ctx.Log.ErrorContext(ctx.RequestCtx, "unable to update check run", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}
}

func (c *ChecksOutputUpdater) handleCommandFailure(ctx *command.Context, cmd PullCommand, res command.Result) {
	updateStatusReq := types.UpdateStatusRequest{
		Repo:             ctx.HeadRepo,
		Ref:              ctx.Pull.HeadCommit,
		PullNum:          ctx.Pull.Num,
		PullCreationTime: ctx.Pull.CreatedAt,
		State:            models.FailedVCSStatus,
		StatusName:       c.buildStatusName(cmd, vcs.StatusTitleOptions{}),
		CommandName:      cmd.CommandName().TitleString(),
		Output:           c.buildOutput(res),
	}

	if _, err := c.VCSClient.UpdateStatus(ctx.RequestCtx, updateStatusReq); err != nil {
		ctx.Log.ErrorContext(ctx.RequestCtx, "unable to update check run", map[string]interface{}{
			"error": err.Error(),
		})
	}
}

func (c *ChecksOutputUpdater) buildJobURL(ctx *command.Context, cmd command.Name, jobID string) string {
	if jobID == "" {
		return ""
	}

	// Only support streaming logs for plan and apply operation for now
	if cmd == command.Plan || cmd == command.Apply {
		jobURL, err := c.JobURLGenerator.GenerateProjectJobURL(jobID)
		if err != nil {
			ctx.Log.ErrorContext(ctx.RequestCtx, fmt.Sprintf("generating job URL %v", err))
		}

		return jobURL
	}
	return ""
}

func (c *ChecksOutputUpdater) buildOutput(res command.Result) string {
	if res.Error != nil {
		return res.Error.Error()
	} else if res.Failure != "" {
		return res.Failure
	}
	return ""
}

func (c *ChecksOutputUpdater) buildStatusName(cmd PullCommand, options vcs.StatusTitleOptions) string {
	commandName := cmd.CommandName()
	return c.TitleBuilder.Build(commandName.String(), options)
}

func (c *ChecksOutputUpdater) resolveState(result command.ProjectResult) models.VCSStatus {
	if result.Error != nil || result.Failure != "" {
		return models.FailedVCSStatus
	}

	return models.SuccessVCSStatus
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
