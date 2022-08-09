package events

import (
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/lyft/feature"
)

type ChecksEnabledPrjCmdContextBuilder struct {
	ProjectCommandContextBuilder

	FeatureAllocator    feature.Allocator
	CommitStatusUpdater CommitStatusUpdater
}

func (c *ChecksEnabledPrjCmdContextBuilder) isChecksEnabled(repo models.Repo, pull models.PullRequest) bool {
	if shouldAllocate, err := c.FeatureAllocator.ShouldAllocate(feature.GithubChecks, feature.FeatureContext{
		RepoName:         repo.FullName,
		PullCreationTime: pull.CreatedAt,
	}); !shouldAllocate || err != nil {
		return false
	}

	return true
}

func (c *ChecksEnabledPrjCmdContextBuilder) BuildProjectContext(
	ctx *command.Context,
	cmdName command.Name,
	prjCfg valid.MergedProjectCfg,
	commentArgs []string,
	repoDir string,
	contextFlags *command.ContextFlags,
) []command.ProjectContext {
	prjCtxs := c.ProjectCommandContextBuilder.BuildProjectContext(ctx, cmdName, prjCfg, commentArgs, repoDir, contextFlags)
	if !c.isChecksEnabled(ctx.HeadRepo, ctx.Pull) || contextFlags.SkipCheckRuns {
		return prjCtxs
	}

	// Create pending checkrun for project commands
	for i, prjCtx := range prjCtxs {

		statusId, _ := c.CommitStatusUpdater.UpdateProject(ctx.RequestCtx, prjCtx, cmdName, models.PendingCommitStatus, "", "")

		prjCtxs[i].CheckRunId = statusId
	}
	return prjCtxs

}

type ProjectCommandContextBuilder interface {
	// BuildProjectContext builds project command contexts for atlantis commands
	BuildProjectContext(
		ctx *command.Context,
		cmdName command.Name,
		prjCfg valid.MergedProjectCfg,
		commentArgs []string,
		repoDir string,
		contextFlags *command.ContextFlags,
	) []command.ProjectContext
}

func NewProjectCommandContextBuilder(
	commentBuilder CommentBuilder,
) ProjectCommandContextBuilder {
	return &projectCommandContextBuilder{
		CommentBuilder: commentBuilder,
	}
}

type projectCommandContextBuilder struct {
	CommentBuilder CommentBuilder
}

func (cb *projectCommandContextBuilder) BuildProjectContext(
	ctx *command.Context,
	cmdName command.Name,
	prjCfg valid.MergedProjectCfg,
	commentArgs []string,
	repoDir string,
	contextFlags *command.ContextFlags,
) []command.ProjectContext {
	return buildContext(
		ctx,
		cmdName,
		getSteps(cmdName, prjCfg.Workflow, contextFlags.LogLevel),
		cb.CommentBuilder,
		prjCfg,
		commentArgs,
		repoDir,
		contextFlags,
	)
}

func buildContext(
	ctx *command.Context,
	cmdName command.Name,
	steps []valid.Step,
	commentBuilder CommentBuilder,
	prjCfg valid.MergedProjectCfg,
	commentArgs []string,
	repoDir string,
	contextFlags *command.ContextFlags,
) []command.ProjectContext {
	projectCmds := make([]command.ProjectContext, 0)

	// If TerraformVersion not defined in config file look for a
	// terraform.require_version block.
	if prjCfg.TerraformVersion == nil {
		prjCfg.TerraformVersion = getTfVersion(ctx, filepath.Join(repoDir, prjCfg.RepoRelDir))
	}

	projectCmds = append(projectCmds,
		command.NewProjectContext(
			ctx,
			cmdName,
			commentBuilder.BuildApplyComment(prjCfg.RepoRelDir, prjCfg.Workspace, prjCfg.Name),
			commentBuilder.BuildPlanComment(prjCfg.RepoRelDir, prjCfg.Workspace, prjCfg.Name, commentArgs),
			prjCfg,
			steps,
			prjCfg.PolicySets,
			escapeArgs(commentArgs),
			contextFlags,
			ctx.Scope,
			ctx.PullRequestStatus,
		),
	)

	return projectCmds

}

func getSteps(
	cmdName command.Name,
	workflow valid.Workflow,
	logLevel string,
) (steps []valid.Step) {
	switch cmdName {
	case command.Plan:
		steps = workflow.Plan.Steps
		if logLevel != "" {
			steps = valid.PrependLogEnvStep(steps, logLevel)
		}
	case command.Apply:
		steps = workflow.Apply.Steps
		if logLevel != "" {
			steps = valid.PrependLogEnvStep(steps, logLevel)
		}
	case command.Version:
		steps = []valid.Step{{
			StepName: "version",
		}}
	case command.PolicyCheck:
		steps = workflow.PolicyCheck.Steps
	}
	return steps
}

func escapeArgs(args []string) []string {
	var escaped []string
	for _, arg := range args {
		var escapedArg string
		for i := range arg {
			escapedArg += "\\" + string(arg[i])
		}
		escaped = append(escaped, escapedArg)
	}
	return escaped
}

// Extracts required_version from Terraform configuration.
// Returns nil if unable to determine version from configuration.
func getTfVersion(ctx *command.Context, absProjDir string) *version.Version {
	module, diags := tfconfig.LoadModule(absProjDir)
	if diags.HasErrors() {
		ctx.Log.ErrorContext(ctx.RequestCtx, fmt.Sprintf("trying to detect required version: %s", diags.Error()))
		return nil
	}
	if len(module.RequiredCore) != 1 {
		ctx.Log.InfoContext(ctx.RequestCtx, fmt.Sprintf("cannot determine which version to use from terraform configuration, detected %d possibilities.", len(module.RequiredCore)))
		return nil
	}
	requiredVersionSetting := module.RequiredCore[0]

	// We allow `= x.y.z`, `=x.y.z` or `x.y.z` where `x`, `y` and `z` are integers.
	re := regexp.MustCompile(`^=?\s*([^\s]+)\s*$`)
	matched := re.FindStringSubmatch(requiredVersionSetting)
	if len(matched) == 0 {
		return nil
	}
	version, err := version.NewVersion(matched[1])
	if err != nil {
		ctx.Log.ErrorContext(ctx.RequestCtx, err.Error())
		return nil
	}

	ctx.Log.InfoContext(ctx.RequestCtx, fmt.Sprintf("detected module requires version: %q", version.String()))
	return version
}
