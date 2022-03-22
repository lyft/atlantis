package events

import (
	"path/filepath"
	"regexp"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/uber-go/tally"
)

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
	policyCheckEnabled bool,
	commentBuilder CommentBuilder,
	scope tally.Scope,
) ProjectCommandContextBuilder {
	var builder ProjectCommandContextBuilder
	builder = &projectCommandContextBuilder{
		PolicyChecksEnabled: policyCheckEnabled,
		CommentBuilder:      commentBuilder,
	}

	builderWithStats := &InstrumentedProjectCommandContextBuilder{
		ProjectCommandContextBuilder: builder,
		ProjectCounter:               scope.Counter("projects"),
	}

	return builderWithStats
}

type projectCommandContextBuilder struct {
	PolicyChecksEnabled bool
	CommentBuilder      CommentBuilder
}

func (cb *projectCommandContextBuilder) BuildProjectContext(
	ctx *command.Context,
	cmdName command.Name,
	prjCfg valid.MergedProjectCfg,
	commentArgs []string,
	repoDir string,
	contextFlags *command.ContextFlags,
) []command.ProjectContext {
	ctx.Log.Debugf("Building project command context for %s", cmdName)
	projectCmds := make([]command.ProjectContext, 0)

	// If TerraformVersion not defined in config file look for a
	// terraform.require_version block.
	if prjCfg.TerraformVersion == nil {
		prjCfg.TerraformVersion = getTfVersion(ctx, filepath.Join(repoDir, prjCfg.RepoRelDir))
	}

	steps := getWorkflows(ctx, cmdName, prjCfg)

	projectCmds = append(projectCmds,
		command.NewProjectContext(
			ctx,
			cmdName,
			cb.CommentBuilder.BuildApplyComment(prjCfg.RepoRelDir, prjCfg.Workspace, prjCfg.Name, prjCfg.AutoMergeDisabled),
			cb.CommentBuilder.BuildPlanComment(prjCfg.RepoRelDir, prjCfg.Workspace, prjCfg.Name, commentArgs),
			cb.CommentBuilder.BuildVersionComment(prjCfg.RepoRelDir, prjCfg.Workspace, prjCfg.Name),
			prjCfg,
			steps,
			prjCfg.PolicySets,
			escapeArgs(commentArgs),
			contextFlags,
			ctx.Scope,
			ctx.PullRequestStatus,
		),
	)

	// When policy checks enabled we want to generate project context for policy
	// check command.
	if cb.PolicyChecksEnabled && cmdName == command.Plan {
		policyCheckCmds := cb.BuildProjectContext(
			ctx,
			command.PolicyCheck,
			prjCfg,
			commentArgs,
			repoDir,
			contextFlags,
		)
		projectCmds = append(projectCmds,
			policyCheckCmds...,
		)
	}

	return projectCmds
}

func getWorkflows(
	ctx *command.Context,
	cmdName command.Name,
	prjCfg valid.MergedProjectCfg,
) (steps []valid.Step) {
	var workflow valid.Workflow

	switch ctx.ExecutionMode {
	case command.DefaultExecutionMode:
		workflow = prjCfg.Workflow
	case command.PullRequestExecutionMode:
		workflow = prjCfg.PullRequestWorkflow
	case command.DeploymentExecutionMode:
		workflow = prjCfg.DeploymentWorkflow
	}

	switch cmdName {
	case command.Plan:
		steps = workflow.Plan.Steps
	case command.Apply:
		steps = workflow.Apply.Steps
	case command.Version:
		steps = []valid.Step{{
			StepName: "version",
		}}
	case command.PolicyCheck:
		steps = workflow.PolicyCheck.Steps
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
func getTfVersion(ctx *command.Context, absProjDir string) *version.Version {
	module, diags := tfconfig.LoadModule(absProjDir)
	if diags.HasErrors() {
		ctx.Log.Errorf("trying to detect required version: %s", diags.Error())
		return nil
	}
	if len(module.RequiredCore) != 1 {
		ctx.Log.Infof("cannot determine which version to use from terraform configuration, detected %d possibilities.", len(module.RequiredCore))
		return nil
	}
	requiredVersionSetting := module.RequiredCore[0]

	// We allow `= x.y.z`, `=x.y.z` or `x.y.z` where `x`, `y` and `z` are integers.
	re := regexp.MustCompile(`^=?\s*([^\s]+)\s*$`)
	matched := re.FindStringSubmatch(requiredVersionSetting)
	if len(matched) == 0 {
		ctx.Log.Debugf("did not specify exact version in terraform configuration, found %q", requiredVersionSetting)
		return nil
	}
	ctx.Log.Debugf("found required_version setting of %q", requiredVersionSetting)
	version, err := version.NewVersion(matched[1])
	if err != nil {
		ctx.Log.Debugf(err.Error())
		return nil
	}

	ctx.Log.Infof("detected module requires version: %q", version.String())
	return version
}
