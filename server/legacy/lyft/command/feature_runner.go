package command

import (
	"github.com/runatlantis/atlantis/server/legacy/events"
	"github.com/runatlantis/atlantis/server/legacy/events/command"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/models"
	"github.com/runatlantis/atlantis/server/neptune/lyft/feature"
	"github.com/runatlantis/atlantis/server/neptune/template"
)

type Commenter interface {
	CreateComment(repo models.Repo, pullNum int, comment string, command string) error
}

type LegacyApplyCommentInput struct{}

type PlatformModeRunner struct {
	command.Runner
	Allocator      feature.Allocator
	Logger         logging.Logger
	Builder        events.ProjectApplyCommandBuilder
	TemplateLoader template.Loader[LegacyApplyCommentInput]
	VCSClient      Commenter
}

func (a *PlatformModeRunner) Run(ctx *command.Context, cmd *command.Comment) {
	if cmd.Name != command.Apply {
		a.Runner.Run(ctx, cmd)
		return
	}

	// now let's determine whether the repo is configured for platform mode by building commands
	var projectCmds []command.ProjectContext
	projectCmds, err := a.Builder.BuildApplyCommands(ctx, cmd)
	if err != nil {
		a.Logger.ErrorContext(ctx.RequestCtx, err.Error())
		return
	}

	// is this possible? Not sure, let's be safe tho and just bail into the delegate
	if len(projectCmds) == 0 {
		a.Logger.WarnContext(ctx.RequestCtx, "no project commands. unable to determine workflow mode type")
		a.Runner.Run(ctx, cmd)
		return
	}

	// at this point we've either commented about this being a legacy apply or not, so let's just proceed with
	// the run now.
	a.Runner.Run(ctx, cmd)
}

// DefaultProjectCommandRunner implements ProjectCommandRunner.
type PlatformModeProjectRunner struct { //create object and test
	PlatformModeRunner events.ProjectCommandRunner
	PrModeRunner       events.ProjectCommandRunner
	Allocator          feature.Allocator
	Logger             logging.Logger
}

// Plan runs terraform plan for the project described by ctx.
func (p *PlatformModeProjectRunner) Plan(ctx command.ProjectContext) command.ProjectResult {
	return p.PlatformModeRunner.Plan(ctx)
}

// PolicyCheck evaluates policies defined with Rego for the project described by ctx.
func (p *PlatformModeProjectRunner) PolicyCheck(ctx command.ProjectContext) command.ProjectResult {
	return p.PlatformModeRunner.PolicyCheck(ctx)
}

// Apply runs terraform apply for the project described by ctx.
func (p *PlatformModeProjectRunner) Apply(ctx command.ProjectContext) command.ProjectResult {
	return command.ProjectResult{
		Command:      command.Apply,
		RepoRelDir:   ctx.RepoRelDir,
		Workspace:    ctx.Workspace,
		ProjectName:  ctx.ProjectName,
		StatusID:     ctx.StatusID,
		ApplySuccess: "atlantis apply is disabled for this project. Please track the deployment when the PR is merged. ",
	}
}

func (p *PlatformModeProjectRunner) Version(ctx command.ProjectContext) command.ProjectResult {
	return p.PlatformModeRunner.Version(ctx)
}
