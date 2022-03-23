package events

import (
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/command"
)

type policyCheckProjectContextBuilder struct {
	ProjectCommandContextBuilder
	CommentBuilder CommentBuilder
}

func (p *policyCheckProjectContextBuilder) BuildProjectContext(
	ctx *command.Context,
	cmdName command.Name,
	prjCfg valid.MergedProjectCfg,
	commentArgs []string,
	repoDir string,
	contextFlags *command.ContextFlags,
) []command.ProjectContext {
	prjCmds := p.ProjectCommandContextBuilder.BuildProjectContext(ctx, cmdName, prjCfg, commentArgs, repoDir, contextFlags)
	if cmdName == command.Plan {
		prjCmds = append(prjCmds,
			buildContext(
				ctx,
				cmdName,
				getSteps(command.PolicyCheck, prjCfg.Workflow),
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
