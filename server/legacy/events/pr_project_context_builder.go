package events

import (
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/legacy/events/command"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/neptune/lyft/feature"
)

func NewPlatformModeProjectCommandContextBuilder(
	commentBuilder CommentBuilder,
	delegate ProjectCommandContextBuilder,
	logger logging.Logger,
	allocator feature.Allocator,
) *PlatformModeProjectContextBuilder {
	return &PlatformModeProjectContextBuilder{
		CommentBuilder: commentBuilder,
		delegate:       delegate,
		Logger:         logger,
		allocator:      allocator,
	}
}

type PlatformModeProjectContextBuilder struct {
	delegate       ProjectCommandContextBuilder
	allocator      feature.Allocator
	CommentBuilder CommentBuilder
	Logger         logging.Logger
}

func (p *PlatformModeProjectContextBuilder) BuildProjectContext(
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
		getSteps(cmdName, prjCfg.PullRequestWorkflow, contextFlags.LogLevel),
		p.CommentBuilder,
		prjCfg,
		commentArgs,
		repoDir,
		contextFlags,
	)
}
