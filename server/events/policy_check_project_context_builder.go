package events

import (
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/command"
)

type PolicyCheckProjectContextBuilder struct {
	ProjectCommandContextBuilder
	CommentBuilder CommentBuilder
}

func (p *PolicyCheckProjectContextBuilder) BuildProjectContext(
	ctx *command.Context,
	cmdName command.Name,
	prjCfg valid.MergedProjectCfg,
	commentArgs []string,
	repoDir string,
	contextFlags *command.ContextFlags,
	logLevel string,
) []command.ProjectContext {
	prjCmds := p.ProjectCommandContextBuilder.BuildProjectContext(ctx, cmdName, prjCfg, commentArgs, repoDir, contextFlags, logLevel)
	if cmdName == command.Plan {
		prjCmds = append(prjCmds,
			buildContext(
				ctx,
				command.PolicyCheck,
				getSteps(command.PolicyCheck, prjCfg.Workflow, logLevel),
				p.CommentBuilder,
				prjCfg,
				commentArgs,
				repoDir,
				contextFlags,
			)...,
		)
	}

	return prjCmds
}
