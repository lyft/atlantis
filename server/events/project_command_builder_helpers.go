package events

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/yaml"
	"github.com/runatlantis/atlantis/server/events/yaml/valid"
)

// ProjectCommandBuilder helper functions

func buildRePlanAndApplyComments(commentBuilder CommentBuilder, repoRelDir string, workspace string, project string, commentArgs ...string) (applyCmd string, planCmd string) {
	applyCmd = commentBuilder.BuildApplyComment(repoRelDir, workspace, project)
	planCmd = commentBuilder.BuildPlanComment(repoRelDir, workspace, project, commentArgs)
	return
}

// validateWorkspaceAllowed returns an error if repoCfg defines projects in
// repoRelDir but none of them use workspace. We want this to be an error
// because if users have gone to the trouble of defining projects in repoRelDir
// then it's likely that if we're running a command for a workspace that isn't
// defined then they probably just typed the workspace name wrong.
func validateWorkspaceAllowed(repoCfg *valid.RepoCfg, repoRelDir string, workspace string) error {
	if repoCfg == nil {
		return nil
	}

	return repoCfg.ValidateWorkspaceAllowed(repoRelDir, workspace)
}

// getCfg returns the atlantis.yaml config (if it exists) for this project. If
// there is no config, then projectCfg and repoCfg will be nil.
func getCfg(
	ctx *CommandContext,
	parserValidator *yaml.ParserValidator,
	globalCfg valid.GlobalCfg,
	projectName string,
	dir string,
	workspace string,
	repoDir string,
) (projectCfg *valid.Project, repoCfg *valid.RepoCfg, err error) {

	hasConfigFile, err := parserValidator.HasRepoCfg(repoDir)
	if err != nil {
		err = errors.Wrapf(err, "looking for %s file in %q", yaml.AtlantisYAMLFilename, repoDir)
		return
	}
	if !hasConfigFile {
		if projectName != "" {
			err = fmt.Errorf("cannot specify a project name unless an %s file exists to configure projects", yaml.AtlantisYAMLFilename)
			return
		}
		return
	}

	var repoConfig valid.RepoCfg
	repoConfig, err = parserValidator.ParseRepoCfg(repoDir, globalCfg, ctx.Pull.BaseRepo.ID())
	if err != nil {
		return
	}
	repoCfg = &repoConfig

	// If they've specified a project by name we look it up. Otherwise we
	// use the dir and workspace.
	if projectName != "" {
		projectCfg = repoCfg.FindProjectByName(projectName)
		if projectCfg == nil {
			err = fmt.Errorf("no project with name %q is defined in %s", projectName, yaml.AtlantisYAMLFilename)
			return
		}
		return
	}

	projCfgs := repoCfg.FindProjectsByDirWorkspace(dir, workspace)
	if len(projCfgs) == 0 {
		return
	}
	if len(projCfgs) > 1 {
		err = fmt.Errorf("must specify project name: more than one project defined in %s matched dir: %q workspace: %q", yaml.AtlantisYAMLFilename, dir, workspace)
		return
	}
	projectCfg = &projCfgs[0]
	return
}
