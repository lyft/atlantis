package builders

import (
	"path/filepath"
	"regexp"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/parsers"
	"github.com/runatlantis/atlantis/server/events/yaml/valid"
)

func NewProjectContextBulder(policyCheckEnabled bool, commentBuilder parsers.CommentBuilder) ProjectCommandContextBuilder {
	projectCommandContextBuilder := &DefaultProjectCommandContextBuilder{
		CommentBuilder: commentBuilder,
	}

	if policyCheckEnabled {
		return &PolicyCheckProjectCommandContextBuilder{
			CommentBuilder:               commentBuilder,
			ProjectCommandContextBuilder: projectCommandContextBuilder,
		}
	}

	return projectCommandContextBuilder
}

// DefaultProjectContextBuilder builds ProjectCommandContext
type ProjectCommandContextBuilder interface {
	BuildProjectContext(
		ctx *models.CommandContext,
		cmdName models.CommandName,
		prjCfg valid.MergedProjectCfg,
		commentFlags []string,
		repoDir string,
		automerge, parallelPlan, parallelApply, verbose bool,
	) []models.ProjectCommandContext
}

type DefaultProjectCommandContextBuilder struct {
	CommentBuilder parsers.CommentBuilder
}

func (cb *DefaultProjectCommandContextBuilder) BuildProjectContext(
	ctx *models.CommandContext,
	cmdName models.CommandName,
	prjCfg valid.MergedProjectCfg,
	commentFlags []string,
	repoDir string,
	automerge, parallelPlan, parallelApply, verbose bool,
) (projectCmds []models.ProjectCommandContext) {
	ctx.Log.Debug("Building project command context for %s", cmdName)

	var policySets models.PolicySets
	var steps []valid.Step
	switch cmdName {
	case models.PlanCommand:
		steps = prjCfg.Workflow.Plan.Steps
	case models.ApplyCommand:
		steps = prjCfg.Workflow.Apply.Steps
	}

	// If TerraformVersion not defined in config file look for a
	// terraform.require_version block.
	if prjCfg.TerraformVersion == nil {
		prjCfg.TerraformVersion = getTfVersion(ctx, filepath.Join(repoDir, prjCfg.RepoRelDir))
	}

	projectCmds = append(projectCmds, models.NewProjectCommandContext(
		ctx,
		cmdName,
		cb.CommentBuilder.BuildApplyComment(prjCfg.RepoRelDir, prjCfg.Workspace, prjCfg.Name),
		cb.CommentBuilder.BuildPlanComment(prjCfg.RepoRelDir, prjCfg.Workspace, prjCfg.Name, commentFlags),
		prjCfg,
		steps,
		policySets,
		escapeArgs(commentFlags),
		automerge,
		parallelApply,
		parallelPlan,
		verbose,
	))

	return
}

type PolicyCheckProjectCommandContextBuilder struct {
	ProjectCommandContextBuilder *DefaultProjectCommandContextBuilder
	CommentBuilder               parsers.CommentBuilder
}

func (cb *PolicyCheckProjectCommandContextBuilder) BuildProjectContext(
	ctx *models.CommandContext,
	cmdName models.CommandName,
	prjCfg valid.MergedProjectCfg,
	commentFlags []string,
	repoDir string,
	automerge, parallelPlan, parallelApply, verbose bool,
) (projectCmds []models.ProjectCommandContext) {
	ctx.Log.Debug("PolicyChecks are enabled")
	ctx.Log.Debug("Building project command context for %s", cmdName)
	projectCmds = cb.ProjectCommandContextBuilder.BuildProjectContext(
		ctx,
		cmdName,
		prjCfg,
		escapeArgs(commentFlags),
		repoDir,
		verbose,
		automerge,
		parallelPlan,
		parallelApply,
	)

	if cmdName == models.PlanCommand {
		var policySets models.PolicySets
		var steps []valid.Step
		steps = prjCfg.Workflow.PolicyCheck.Steps

		projectCmds = append(projectCmds, models.NewProjectCommandContext(
			ctx,
			models.PolicyCheckCommand,
			cb.CommentBuilder.BuildApplyComment(prjCfg.RepoRelDir, prjCfg.Workspace, prjCfg.Name),
			cb.CommentBuilder.BuildPlanComment(prjCfg.RepoRelDir, prjCfg.Workspace, prjCfg.Name, commentFlags),
			prjCfg,
			steps,
			policySets,
			escapeArgs(commentFlags),
			automerge,
			parallelApply,
			parallelPlan,
			verbose,
		))
	}

	return
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
func getTfVersion(ctx *models.CommandContext, absProjDir string) *version.Version {
	module, diags := tfconfig.LoadModule(absProjDir)
	if diags.HasErrors() {
		ctx.Log.Err("trying to detect required version: %s", diags.Error())
		return nil
	}

	if len(module.RequiredCore) != 1 {
		ctx.Log.Info("cannot determine which version to use from terraform configuration, detected %d possibilities.", len(module.RequiredCore))
		return nil
	}
	requiredVersionSetting := module.RequiredCore[0]

	// We allow `= x.y.z`, `=x.y.z` or `x.y.z` where `x`, `y` and `z` are integers.
	re := regexp.MustCompile(`^=?\s*([^\s]+)\s*$`)
	matched := re.FindStringSubmatch(requiredVersionSetting)
	if len(matched) == 0 {
		ctx.Log.Debug("did not specify exact version in terraform configuration, found %q", requiredVersionSetting)
		return nil
	}
	ctx.Log.Debug("found required_version setting of %q", requiredVersionSetting)
	version, err := version.NewVersion(matched[1])
	if err != nil {
		ctx.Log.Debug(err.Error())
		return nil
	}

	ctx.Log.Info("detected module requires version: %q", version.String())
	return version
}
