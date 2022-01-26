package decorators

import (
	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/yaml/valid"
	"github.com/runatlantis/atlantis/server/logging"
	"path/filepath"
	"strings"
)

type DestroyPlanProjectFinderWrapper struct {
	events.ProjectFinder
}

func (d DestroyPlanProjectFinderWrapper) DetermineProjectsViaConfig(log logging.SimpleLogging, modifiedFiles []string, config valid.RepoCfg, absRepoDir string) ([]valid.Project, error) {
	if len(config.Projects) == 0 {
		return d.ProjectFinder.DetermineProjectsViaConfig(log, modifiedFiles, config, absRepoDir)
	}
	var allFiles []string
	for _, project := range config.Projects {
		// If we have enabled destroy plan mode, we need to use when_modified to target and delete up all project files
		if project.Tags[PlanMode] == Destroy {
			var whenModifiedRelToRepoRoot []string
			for _, wm := range project.Autoplan.WhenModified {
				wm = strings.TrimSpace(wm)
				// An exclusion uses a '!' at the beginning. If it's there, we need
				// to remove it, then add in the project path, then add it back.
				exclusion := false
				if wm != "" && wm[0] == '!' {
					wm = wm[1:]
					exclusion = true
				}

				// Prepend project dir to when modified patterns because the patterns
				// are relative to the project dirs but our list of modified files is
				// relative to the repo root.
				wmRelPath := filepath.Join(project.Dir, wm)
				if exclusion {
					wmRelPath = "!" + wmRelPath
				}
				whenModifiedRelToRepoRoot = append(whenModifiedRelToRepoRoot, wmRelPath)
			}
			allFiles = append(allFiles, whenModifiedRelToRepoRoot...)
		}
	}
	return d.ProjectFinder.DetermineProjectsViaConfig(log, allFiles, config, absRepoDir)
}
