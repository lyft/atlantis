package models

import (
	"github.com/runatlantis/atlantis/server/events/yaml/valid"
)

// buildCtx is a helper method that handles constructing the ProjectCommandContext.
func NewProjectCommandContext(ctx *CommandContext,
	cmd CommandName,
	applyCmd string,
	planCmd string,
	projCfg valid.MergedProjectCfg,
	steps []valid.Step,
	policySets PolicySets,
	escapedCommentArgs []string,
	automergeEnabled bool,
	parallelApplyEnabled bool,
	parallelPlanEnabled bool,
	verbose bool,
) ProjectCommandContext {
	return ProjectCommandContext{
		CommandName:          cmd,
		ApplyCmd:             applyCmd,
		BaseRepo:             ctx.Pull.BaseRepo,
		EscapedCommentArgs:   escapedCommentArgs,
		AutomergeEnabled:     automergeEnabled,
		ParallelApplyEnabled: parallelApplyEnabled,
		ParallelPlanEnabled:  parallelPlanEnabled,
		AutoplanEnabled:      projCfg.AutoplanEnabled,
		Steps:                steps,
		HeadRepo:             ctx.HeadRepo,
		Log:                  ctx.Log,
		PullMergeable:        ctx.PullMergeable,
		Pull:                 ctx.Pull,
		ProjectName:          projCfg.Name,
		ApplyRequirements:    projCfg.ApplyRequirements,
		RePlanCmd:            planCmd,
		RepoRelDir:           projCfg.RepoRelDir,
		RepoConfigVersion:    projCfg.RepoCfgVersion,
		TerraformVersion:     projCfg.TerraformVersion,
		User:                 ctx.User,
		Verbose:              verbose,
		Workspace:            projCfg.Workspace,
		PolicySets:           policySets,
	}
}
