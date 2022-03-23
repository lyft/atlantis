package events

import (
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/uber-go/tally"
)

func NewPRProjectCommandContextBuilder(
	policyCheckEnabled bool,
	commentBuilder CommentBuilder,
	scope tally.Scope,
) ProjectCommandContextBuilder {
	var builder ProjectCommandContextBuilder
	builder = &prProjectContextBuilder{
		CommentBuilder: commentBuilder,
	}

	if policyCheckEnabled {
		builder = &policyCheckProjectContextBuilder{
			ProjectCommandContextBuilder: builder,
			CommentBuilder:               commentBuilder,
		}
	}

	builder = &InstrumentedProjectCommandContextBuilder{
		ProjectCommandContextBuilder: builder,
		ProjectCounter:               scope.Counter("projects"),
	}

	return builder
}

type prProjectContextBuilder struct {
	ProjectCommandContextBuilder
	CommentBuilder CommentBuilder
}

func (p *prProjectContextBuilder) BuildProjectContext(
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
		getSteps(cmdName, prjCfg.Workflow),
		p.CommentBuilder,
		prjCfg,
		commentArgs,
		repoDir,
		contextFlags,
	)
}
