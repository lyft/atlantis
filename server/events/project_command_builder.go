package events

import (
	"os"

	"github.com/runatlantis/atlantis/server/events/builders"
	"github.com/runatlantis/atlantis/server/events/defaults"
	"github.com/runatlantis/atlantis/server/events/parsers"
	"github.com/runatlantis/atlantis/server/events/yaml/valid"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/events/yaml"
)

func NewProjectCommandBuilder(
	policyChecksSupported bool,
	parserValidator *yaml.ParserValidator,
	projectFinder ProjectFinder,
	vcsClient vcs.Client,
	workingDir WorkingDir,
	workingDirLocker WorkingDirLocker,
	globalCfg valid.GlobalCfg,
	pendingPlanFinder *DefaultPendingPlanFinder,
	commentBuilder parsers.CommentBuilder,
	skipCloneNoChanges bool,
) *DefaultProjectCommandBuilder {
	projectCommandBuilder := &DefaultProjectCommandBuilder{
		ParserValidator:    parserValidator,
		ProjectFinder:      projectFinder,
		VCSClient:          vcsClient,
		WorkingDir:         workingDir,
		WorkingDirLocker:   workingDirLocker,
		GlobalCfg:          globalCfg,
		PendingPlanFinder:  pendingPlanFinder,
		SkipCloneNoChanges: skipCloneNoChanges,
		ProjectCommandContextBuilder: builders.NewProjectContextBulder(
			policyChecksSupported,
			commentBuilder,
		),
	}

	return projectCommandBuilder
}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_project_command_builder.go ProjectCommandBuilder

// ProjectCommandBuilder builds commands that run on individual projects.
type ProjectCommandBuilder interface {
	// BuildAutoplanCommands builds project commands that will run plan on
	// the projects determined to be modified.
	BuildAutoplanCommands(ctx *models.CommandContext) ([]models.ProjectCommandContext, error)
	// BuildPlanCommands builds project plan commands for this ctx and comment. If
	// comment doesn't specify one project then there may be multiple commands
	// to be run.
	BuildPlanCommands(ctx *models.CommandContext, comment *parsers.CommentCommand) ([]models.ProjectCommandContext, error)
	// BuildApplyCommands builds project apply commands for ctx and comment. If
	// comment doesn't specify one project then there may be multiple commands
	// to be run.
	BuildApplyCommands(ctx *models.CommandContext, comment *parsers.CommentCommand) ([]models.ProjectCommandContext, error)
}

// DefaultProjectCommandBuilder implements ProjectCommandBuilder.
// This class combines the data from the comment and any atlantis.yaml file or
// Atlantis server config and then generates a set of contexts.
type DefaultProjectCommandBuilder struct {
	ParserValidator              *yaml.ParserValidator
	ProjectFinder                ProjectFinder
	VCSClient                    vcs.Client
	WorkingDir                   WorkingDir
	WorkingDirLocker             WorkingDirLocker
	GlobalCfg                    valid.GlobalCfg
	PendingPlanFinder            *DefaultPendingPlanFinder
	ProjectCommandContextBuilder builders.ProjectCommandContextBuilder
	SkipCloneNoChanges           bool
}

// See ProjectCommandBuilder.BuildAutoplanCommands.
func (p *DefaultProjectCommandBuilder) BuildAutoplanCommands(ctx *models.CommandContext) ([]models.ProjectCommandContext, error) {
	projCtxs, err := p.buildPlanAllCommands(ctx, nil, false)
	if err != nil {
		return nil, err
	}
	var autoplanEnabled []models.ProjectCommandContext
	for _, projCtx := range projCtxs {
		if !projCtx.AutoplanEnabled {
			ctx.Log.Debug("ignoring project at dir %q, workspace: %q because autoplan is disabled", projCtx.RepoRelDir, projCtx.Workspace)
			continue
		}
		autoplanEnabled = append(autoplanEnabled, projCtx)
	}
	return autoplanEnabled, nil
}

// See ProjectCommandBuilder.BuildPlanCommands.
func (p *DefaultProjectCommandBuilder) BuildPlanCommands(ctx *models.CommandContext, cmd *parsers.CommentCommand) ([]models.ProjectCommandContext, error) {
	if !cmd.IsForSpecificProject() {
		return p.buildPlanAllCommands(ctx, cmd.Flags, cmd.Verbose)
	}
	pcc, err := p.buildProjectPlanCommand(ctx, cmd)
	return pcc, err
}

// See ProjectCommandBuilder.BuildApplyCommands.
func (p *DefaultProjectCommandBuilder) BuildApplyCommands(ctx *models.CommandContext, cmd *parsers.CommentCommand) ([]models.ProjectCommandContext, error) {
	if !cmd.IsForSpecificProject() {
		return p.buildApplyAllCommands(ctx, cmd)
	}
	pac, err := p.buildProjectApplyCommand(ctx, cmd)
	return pac, err
}

// buildPlanAllCommands builds plan contexts for all projects we determine were
// modified in this ctx.
func (p *DefaultProjectCommandBuilder) buildPlanAllCommands(ctx *models.CommandContext, commentFlags []string, verbose bool) ([]models.ProjectCommandContext, error) {
	// We'll need the list of modified files.
	modifiedFiles, err := p.VCSClient.GetModifiedFiles(ctx.Pull.BaseRepo, ctx.Pull)
	if err != nil {
		return nil, err
	}
	ctx.Log.Debug("%d files were modified in this pull request", len(modifiedFiles))

	if p.SkipCloneNoChanges && p.VCSClient.SupportsSingleFileDownload(ctx.Pull.BaseRepo) {
		hasRepoCfg, repoCfgData, err := p.VCSClient.DownloadRepoConfigFile(ctx.Pull)
		if err != nil {
			return nil, errors.Wrapf(err, "downloading %s", yaml.AtlantisYAMLFilename)
		}

		if hasRepoCfg {
			repoCfg, err := p.ParserValidator.ParseRepoCfgData(repoCfgData, p.GlobalCfg, ctx.Pull.BaseRepo.ID())
			if err != nil {
				return nil, errors.Wrapf(err, "parsing %s", yaml.AtlantisYAMLFilename)
			}
			ctx.Log.Info("successfully parsed remote %s file", yaml.AtlantisYAMLFilename)
			matchingProjects, err := p.ProjectFinder.DetermineProjectsViaConfig(ctx.Log, modifiedFiles, repoCfg, "")
			if err != nil {
				return nil, err
			}
			ctx.Log.Info("%d projects are changed on MR %q based on their when_modified config", len(matchingProjects), ctx.Pull.Num)
			if len(matchingProjects) == 0 {
				ctx.Log.Info("skipping repo clone since no project was modified")
				return []models.ProjectCommandContext{}, nil
			}
			// NOTE: We discard this work here and end up doing it again after
			// cloning to ensure all the return values are set properly with
			// the actual clone directory.
		}
	}

	// Need to lock the workspace we're about to clone to.
	workspace := defaults.DefaultWorkspace

	unlockFn, err := p.WorkingDirLocker.TryLock(ctx.Pull.BaseRepo.FullName, ctx.Pull.Num, workspace)
	if err != nil {
		ctx.Log.Warn("workspace was locked")
		return nil, err
	}
	ctx.Log.Debug("got workspace lock")
	defer unlockFn()

	repoDir, _, err := p.WorkingDir.Clone(ctx.Log, ctx.HeadRepo, ctx.Pull, workspace)
	if err != nil {
		return nil, err
	}

	// Parse config file if it exists.
	hasRepoCfg, err := p.ParserValidator.HasRepoCfg(repoDir)
	if err != nil {
		return nil, errors.Wrapf(err, "looking for %s file in %q", yaml.AtlantisYAMLFilename, repoDir)
	}

	var projCtxs []models.ProjectCommandContext

	// prjConfigBuilder := &yaml.ProjectConfigBuilder{
	// 	parser:    p.ParserValidator,
	// 	globalCfg: p.GlobalCfg,
	// }
	// p.ProjectConfigBuilder.GetProjectConfigs(ctx, repoDir, modifiedFiles)

	if hasRepoCfg {
		// If there's a repo cfg then we'll use it to figure out which projects
		// should be planed.
		repoCfg, err := p.ParserValidator.ParseRepoCfg(repoDir, p.GlobalCfg, ctx.Pull.BaseRepo.ID())
		if err != nil {
			return nil, errors.Wrapf(err, "parsing %s", yaml.AtlantisYAMLFilename)
		}
		ctx.Log.Info("successfully parsed %s file", yaml.AtlantisYAMLFilename)
		matchingProjects, err := p.ProjectFinder.DetermineProjectsViaConfig(ctx.Log, modifiedFiles, repoCfg, repoDir)
		if err != nil {
			return nil, err
		}
		ctx.Log.Info("%d projects are to be planned based on their when_modified config", len(matchingProjects))

		for _, mp := range matchingProjects {
			ctx.Log.Debug("determining config for project at dir: %q workspace: %q", mp.Dir, mp.Workspace)
			mergedCfg := p.GlobalCfg.MergeProjectCfg(ctx.Log, ctx.Pull.BaseRepo.ID(), mp, repoCfg)

			projCtxs = append(projCtxs,
				p.ProjectCommandContextBuilder.BuildProjectContext(
					ctx,
					models.PlanCommand,
					mergedCfg,
					commentFlags,
					repoDir,
					repoCfg.Automerge,
					repoCfg.ParallelApply,
					repoCfg.ParallelPlan,
					verbose,
				)...)
		}
	} else {
		// If there is no config file, then we'll plan each project that
		// our algorithm determines was modified.
		ctx.Log.Info("found no %s file", yaml.AtlantisYAMLFilename)
		modifiedProjects := p.ProjectFinder.DetermineProjects(ctx.Log, modifiedFiles, ctx.Pull.BaseRepo.FullName, repoDir)
		ctx.Log.Info("automatically determined that there were %d projects modified in this pull request: %s", len(modifiedProjects), modifiedProjects)
		for _, mp := range modifiedProjects {
			ctx.Log.Debug("determining config for project at dir: %q", mp.Path)
			pCfg := p.GlobalCfg.DefaultProjCfg(ctx.Log, ctx.Pull.BaseRepo.ID(), mp.Path, defaults.DefaultWorkspace)

			projCtxs = append(projCtxs,
				p.ProjectCommandContextBuilder.BuildProjectContext(
					ctx,
					models.PlanCommand,
					pCfg,
					commentFlags,
					repoDir,
					defaults.DefaultAutomergeEnabled,
					defaults.DefaultParallelApplyEnabled,
					defaults.DefaultParallelPlanEnabled,
					verbose,
				)...)
		}
	}

	return projCtxs, nil
}

// buildProjectPlanCommand builds a plan context for a single project.
// cmd must be for only one project.
func (p *DefaultProjectCommandBuilder) buildProjectPlanCommand(ctx *models.CommandContext, cmd *parsers.CommentCommand) ([]models.ProjectCommandContext, error) {
	workspace := defaults.DefaultWorkspace
	if cmd.Workspace != "" {
		workspace = cmd.Workspace
	}

	var pcc []models.ProjectCommandContext
	ctx.Log.Debug("building plan command")
	unlockFn, err := p.WorkingDirLocker.TryLock(ctx.Pull.BaseRepo.FullName, ctx.Pull.Num, workspace)
	if err != nil {
		return pcc, err
	}
	defer unlockFn()

	ctx.Log.Debug("cloning repository")
	repoDir, _, err := p.WorkingDir.Clone(ctx.Log, ctx.HeadRepo, ctx.Pull, workspace)
	if err != nil {
		return pcc, err
	}

	repoRelDir := defaults.DefaultRepoRelDir
	if cmd.RepoRelDir != "" {
		repoRelDir = cmd.RepoRelDir
	}

	return p.buildProjectCommandCtx(
		ctx,
		models.PlanCommand,
		cmd.ProjectName,
		cmd.Flags,
		repoDir,
		repoRelDir,
		workspace,
		cmd.Verbose,
	)
}

// buildApplyAllCommands builds contexts for apply for every project that has
// pending plans in this ctx.
func (p *DefaultProjectCommandBuilder) buildApplyAllCommands(ctx *models.CommandContext, commentCmd *parsers.CommentCommand) ([]models.ProjectCommandContext, error) {
	// Lock all dirs in this pull request (instead of a single dir) because we
	// don't know how many dirs we'll need to apply in.
	unlockFn, err := p.WorkingDirLocker.TryLockPull(ctx.Pull.BaseRepo.FullName, ctx.Pull.Num)
	if err != nil {
		return nil, err
	}
	defer unlockFn()

	pullDir, err := p.WorkingDir.GetPullDir(ctx.Pull.BaseRepo, ctx.Pull)
	if err != nil {
		return nil, err
	}

	plans, err := p.PendingPlanFinder.Find(pullDir)
	if err != nil {
		return nil, err
	}

	var cmds []models.ProjectCommandContext
	for _, plan := range plans {
		applyCmds, err := p.buildProjectCommandCtx(ctx, models.ApplyCommand, plan.ProjectName, commentCmd.Flags, plan.RepoDir, plan.RepoRelDir, plan.Workspace, commentCmd.Verbose)
		if err != nil {
			return nil, errors.Wrapf(err, "building command for dir %q", plan.RepoRelDir)
		}
		cmds = append(cmds, applyCmds...)
	}
	return cmds, nil
}

// buildProjectApplyCommand builds an apply command for the single project
// identified by cmd.
func (p *DefaultProjectCommandBuilder) buildProjectApplyCommand(ctx *models.CommandContext, cmd *parsers.CommentCommand) ([]models.ProjectCommandContext, error) {
	workspace := defaults.DefaultWorkspace
	if cmd.Workspace != "" {
		workspace = cmd.Workspace
	}

	var projCtx []models.ProjectCommandContext
	unlockFn, err := p.WorkingDirLocker.TryLock(ctx.Pull.BaseRepo.FullName, ctx.Pull.Num, workspace)
	if err != nil {
		return projCtx, err
	}
	defer unlockFn()

	repoDir, err := p.WorkingDir.GetWorkingDir(ctx.Pull.BaseRepo, ctx.Pull, workspace)
	if os.IsNotExist(errors.Cause(err)) {
		return projCtx, errors.New("no working directory found–did you run plan?")
	} else if err != nil {
		return projCtx, err
	}

	repoRelDir := defaults.DefaultRepoRelDir
	if cmd.RepoRelDir != "" {
		repoRelDir = cmd.RepoRelDir
	}

	return p.buildProjectCommandCtx(
		ctx,
		models.ApplyCommand,
		cmd.ProjectName,
		cmd.Flags,
		repoDir,
		repoRelDir,
		workspace,
		cmd.Verbose,
	)
}

// buildProjectCommandCtx builds a context for a single project identified
// by the parameters.
func (p *DefaultProjectCommandBuilder) buildProjectCommandCtx(ctx *models.CommandContext,
	cmd models.CommandName,
	projectName string,
	commentFlags []string,
	repoDir string,
	repoRelDir string,
	workspace string,
	verbose bool) ([]models.ProjectCommandContext, error) {

	projCfgPtr, repoCfgPtr, err := getCfg(ctx, p.ParserValidator, p.GlobalCfg, projectName, repoRelDir, workspace, repoDir)
	if err != nil {
		return []models.ProjectCommandContext{}, err
	}

	var projCfg valid.MergedProjectCfg
	if projCfgPtr != nil {
		// Override any dir/workspace defined on the comment with what was
		// defined in config. This shouldn't matter since we don't allow comments
		// with both project name and dir/workspace.
		repoRelDir = projCfg.RepoRelDir
		workspace = projCfg.Workspace
		projCfg = p.GlobalCfg.MergeProjectCfg(ctx.Log, ctx.Pull.BaseRepo.ID(), *projCfgPtr, *repoCfgPtr)
	} else {
		projCfg = p.GlobalCfg.DefaultProjCfg(ctx.Log, ctx.Pull.BaseRepo.ID(), repoRelDir, workspace)
	}

	if err := validateWorkspaceAllowed(repoCfgPtr, repoRelDir, workspace); err != nil {
		return []models.ProjectCommandContext{}, err
	}

	automerge := defaults.DefaultAutomergeEnabled
	parallelApply := defaults.DefaultParallelApplyEnabled
	parallelPlan := defaults.DefaultParallelPlanEnabled
	if repoCfgPtr != nil {
		automerge = repoCfgPtr.Automerge
		parallelApply = repoCfgPtr.ParallelApply
		parallelPlan = repoCfgPtr.ParallelPlan
	}

	return p.ProjectCommandContextBuilder.BuildProjectContext(
		ctx,
		cmd,
		projCfg,
		commentFlags,
		repoDir,
		automerge,
		parallelApply,
		parallelPlan,
		verbose,
	), nil
}
