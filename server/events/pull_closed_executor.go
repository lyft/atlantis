// Copyright 2017 HootSuite Media Inc.
//
// Licensed under the Apache License, Version 2.0 (the License);
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an AS IS BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// Modified hereafter by contributors to runatlantis/atlantis.

package events

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/template"

	"github.com/runatlantis/atlantis/server/events/db"
	"github.com/runatlantis/atlantis/server/events/yaml/valid"

	"github.com/runatlantis/atlantis/server/logging"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/locking"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/events/yaml"
	"github.com/runatlantis/atlantis/server/handlers"
)

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_pull_cleaner.go PullCleaner

// PullCleaner cleans up pull requests after they're closed/merged.
type PullCleaner interface {
	// CleanUpPull deletes the workspaces used by the pull request on disk
	// and deletes any locks associated with this pull request for all workspaces.
	CleanUpPull(repo models.Repo, pull models.PullRequest) error
}

// PullClosedExecutor executes the tasks required to clean up a closed pull
// request.
type PullClosedExecutor struct {
	Locker                   locking.Locker
	VCSClient                vcs.Client
	WorkingDir               WorkingDir
	Logger                   logging.SimpleLogging
	DB                       *db.BoltDB
	PullClosedTemplate       PullCleanupTemplate
	LogStreamResourceCleaner handlers.ResourceCleaner
	VCSCient                 vcs.Client
	GlobalCfg                valid.GlobalCfg
	ProjectFinder            ProjectFinder
	AutoplanFileList         string
	ParserVarlidator         yaml.IParserValidator
}

type templatedProject struct {
	RepoRelDir string
	Workspaces string
}

type PullCleanupTemplate interface {
	Execute(wr io.Writer, data interface{}) error
}

type PullClosedEventTemplate struct{}

func (t *PullClosedEventTemplate) Execute(wr io.Writer, data interface{}) error {
	return pullClosedTemplate.Execute(wr, data)
}

var pullClosedTemplate = template.Must(template.New("").Parse(
	"Locks and plans deleted for the projects and workspaces modified in this pull request:\n" +
		"{{ range . }}\n" +
		"- dir: `{{ .RepoRelDir }}` {{ .Workspaces }}{{ end }}"))

// CleanUpPull cleans up after a closed pull request.
func (p *PullClosedExecutor) CleanUpPull(repo models.Repo, pull models.PullRequest) error {
	pullInfoList, err := p.findMatchingProjects(repo, pull)
	if err != nil {
		// Log and continue to clean up other resources.
		p.Logger.Err("retrieving matching projects: %s", err)
	}

	// Clean up logstreaming resources for all projects.
	for _, pullInfo := range pullInfoList {
		p.LogStreamResourceCleaner.CleanUp(pullInfo)
	}

	if err := p.WorkingDir.Delete(repo, pull); err != nil {
		return errors.Wrap(err, "cleaning workspace")
	}

	// Finally, delete locks. We do this last because when someone
	// unlocks a project, right now we don't actually delete the plan
	// so we might have plans laying around but no locks.
	locks, err := p.Locker.UnlockByPull(repo.FullName, pull.Num)
	if err != nil {
		return errors.Wrap(err, "cleaning up locks")
	}

	// Delete pull from DB.
	if err := p.DB.DeletePullStatus(pull); err != nil {
		p.Logger.Err("deleting pull from db: %s", err)
	}

	// If there are no locks then there's no need to comment.
	if len(locks) == 0 {
		return nil
	}

	templateData := p.buildTemplateData(locks)
	var buf bytes.Buffer
	if err = p.PullClosedTemplate.Execute(&buf, templateData); err != nil {
		return errors.Wrap(err, "rendering template for comment")
	}
	return p.VCSClient.CreateComment(repo, pull.Num, buf.String(), "")
}

// buildTemplateData formats the lock data into a slice that can easily be
// templated for the VCS comment. We organize all the workspaces by their
// respective project paths so the comment can look like:
// dir: {dir}, workspaces: {all-workspaces}
func (p *PullClosedExecutor) buildTemplateData(locks []models.ProjectLock) []templatedProject {
	workspacesByPath := make(map[string][]string)
	for _, l := range locks {
		path := l.Project.Path
		workspacesByPath[path] = append(workspacesByPath[path], l.Workspace)
	}

	// sort keys so we can write deterministic tests
	var sortedPaths []string
	for p := range workspacesByPath {
		sortedPaths = append(sortedPaths, p)
	}
	sort.Strings(sortedPaths)

	var projects []templatedProject
	for _, p := range sortedPaths {
		workspace := workspacesByPath[p]
		workspacesStr := fmt.Sprintf("`%s`", strings.Join(workspace, "`, `"))
		if len(workspace) == 1 {
			projects = append(projects, templatedProject{
				RepoRelDir: p,
				Workspaces: "workspace: " + workspacesStr,
			})
		} else {
			projects = append(projects, templatedProject{
				RepoRelDir: p,
				Workspaces: "workspaces: " + workspacesStr,
			})

		}
	}
	return projects
}

func (p *PullClosedExecutor) findMatchingProjects(repo models.Repo, pull models.PullRequest) ([]string, error) {

	var pullInfoList []string
	repoDir, err := p.WorkingDir.GetWorkingDir(repo, pull, "default")
	if err != nil {
		return pullInfoList, errors.Wrap(err, "retrieving working dir")
	}

	modifiedFiles, err := p.VCSClient.GetModifiedFiles(repo, pull)
	if err != nil {
		return pullInfoList, err
	}

	hasRepoCfg, err := p.ParserVarlidator.HasRepoCfg(repoDir)
	if err != nil {
		return pullInfoList, errors.Wrapf(err, "looking for %s file in %q", yaml.AtlantisYAMLFilename, repoDir)
	}

	if hasRepoCfg {
		repoCfg, err := p.ParserVarlidator.ParseRepoCfg(repoDir, p.GlobalCfg, repo.ID())
		if err != nil {
			return pullInfoList, errors.Wrapf(err, "parsing %s", yaml.AtlantisYAMLFilename)
		}

		matchingProjects, err := p.ProjectFinder.DetermineProjectsViaConfig(p.Logger, modifiedFiles, repoCfg, repoDir)
		if err != nil {
			return pullInfoList, err
		}

		for _, project := range matchingProjects {
			pullInfoList = append(pullInfoList, fmt.Sprintf("%s/%d/%s", pull.BaseRepo.FullName, pull.Num, *project.Name))
		}
	} else {
		modifiedProjects := p.ProjectFinder.DetermineProjects(p.Logger, modifiedFiles, repo.FullName, repoDir, p.AutoplanFileList)
		if err != nil {
			return pullInfoList, errors.Wrapf(err, "finding modified projects: %s", modifiedFiles)
		}
		for _, project := range modifiedProjects {
			pullInfoList = append(pullInfoList, fmt.Sprintf("%s/%d/%s", pull.BaseRepo.FullName, pull.Num, project.Path))
		}
	}

	return pullInfoList, nil
}
