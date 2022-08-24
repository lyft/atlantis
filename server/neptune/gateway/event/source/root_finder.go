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

package source

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/runatlantis/atlantis/server/core/config/valid"

	"github.com/docker/docker/pkg/fileutils"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
)

// RootFinder determines which projects were modified in a given event.
type RootFinder interface {
	// DetermineRoots returns the list of projects that were modified based on
	// the modifiedFiles. The list will be de-duplicated.
	// absRepoDir is the path to the cloned repo on disk.
	DetermineRoots(modifiedFiles []string, repoFullName string, absRepoDir string, autoplanFileList string) []models.Project
	// DetermineRootsViaConfig returns the list of projects that were modified
	// based on modifiedFiles and the repo's config.
	// absRepoDir is the path to the cloned repo on disk.
	DetermineRootsViaConfig(modifiedFiles []string, config valid.RepoCfg, absRepoDir string) ([]valid.Project, error)
}

// ignoredFilenameFragments contains filename fragments to ignore while looking at changes
var ignoredFilenameFragments = []string{"terraform.tfstate", "terraform.tfstate.backup", "tflint.hcl"}

// PushEventRootFinder implements RootFinder.
type PushEventRootFinder struct{}

// See PushEventRootFinder.DetermineRoots.
func (p *PushEventRootFinder) DetermineRoots(modifiedFiles []string, repoFullName string, absRepoDir string, autoplanFileList string) []models.Project {
	var roots []models.Project

	modifiedTerraformFiles := p.filterToFileList(modifiedFiles, autoplanFileList)
	if len(modifiedTerraformFiles) == 0 {
		return roots
	}

	var dirs []string
	for _, modifiedFile := range modifiedTerraformFiles {
		rootDir := p.getRootDir(modifiedFile, absRepoDir)
		if rootDir != "" {
			dirs = append(dirs, rootDir)
		}
	}
	uniqueDirs := p.unique(dirs)

	// The list of modified files will include files that were deleted. We still
	// want to run plan if a file was deleted since that often results in a
	// change however we want to remove directories that have been completely
	// deleted.
	exists := p.removeNonExistingDirs(uniqueDirs, absRepoDir)

	for _, p := range exists {
		roots = append(roots, models.NewProject(repoFullName, p))
	}
	return roots
}

// See RootFinder.DetermineRootsViaConfig.
func (p *PushEventRootFinder) DetermineRootsViaConfig(modifiedFiles []string, config valid.RepoCfg, absRepoDir string) ([]valid.Project, error) {
	var roots []valid.Project
	for _, root := range config.Projects {
		var whenModifiedRelToRepoRoot []string
		for _, wm := range root.Autoplan.WhenModified {
			wm = strings.TrimSpace(wm)
			// An exclusion uses a '!' at the beginning. If it's there, we need
			// to remove it, then add in the project path, then add it back.
			exclusion := false
			if wm != "" && wm[0] == '!' {
				wm = wm[1:]
				exclusion = true
			}

			// Prepend root dir to when modified patterns because the patterns
			// are relative to the root dirs but our list of modified files is
			// relative to the repo root.
			wmRelPath := filepath.Join(root.Dir, wm)
			if exclusion {
				wmRelPath = "!" + wmRelPath
			}
			whenModifiedRelToRepoRoot = append(whenModifiedRelToRepoRoot, wmRelPath)
		}
		pm, err := fileutils.NewPatternMatcher(whenModifiedRelToRepoRoot)
		if err != nil {
			return nil, errors.Wrapf(err, "matching modified files with patterns: %v", root.Autoplan.WhenModified)
		}

		// If any of the modified files matches the pattern then this root is
		// considered modified.
		for _, file := range modifiedFiles {
			match, err := pm.Matches(file)
			if err != nil {
				continue
			}
			if match {
				roots = append(roots, root)
				break
			}
		}
	}
	return roots, nil
}

// filterToFileList filters out files not included in the file list
func (p *PushEventRootFinder) filterToFileList(files []string, fileList string) []string {
	var filtered []string
	patterns := strings.Split(fileList, ",")
	// Ignore pattern matcher error here as it was checked for errors in server validation
	patternMatcher, _ := fileutils.NewPatternMatcher(patterns)

	for _, fileName := range files {
		if p.shouldIgnore(fileName) {
			continue
		}
		match, err := patternMatcher.Matches(fileName)
		if err != nil {
			continue
		}
		if match {
			filtered = append(filtered, fileName)
		}
	}

	return filtered
}

// shouldIgnore returns true if we shouldn't trigger a plan on changes to this file.
func (p *PushEventRootFinder) shouldIgnore(fileName string) bool {
	for _, s := range ignoredFilenameFragments {
		if strings.Contains(fileName, s) {
			return true
		}
	}
	return false
}

// getRootDir attempts to determine based on the location of a modified
// file, where the root of the Terraform project is. It also attempts to verify
// if the root is valid by looking for a main.tf file. It returns a relative
// path to the repo. If the project is at the root returns ".". If modified file
// doesn't lead to a valid project path, returns an empty string.
func (p *PushEventRootFinder) getRootDir(modifiedFilePath string, repoDir string) string {
	dir := path.Dir(modifiedFilePath)
	if path.Base(dir) == "env" {
		// If the modified file was inside an env/ directory, we treat this
		// specially and run plan one level up. This supports directory structures
		// like:
		// root/
		//   main.tf
		//   env/
		//     dev.tfvars
		//     staging.tfvars
		return path.Dir(dir)
	}

	// Surrounding dir with /'s so we can match on /modules/ even if dir is
	// "modules" or "project1/modules"
	if strings.Contains("/"+dir+"/", "/modules/") {
		// We treat changes inside modules/ folders specially. There are two cases:
		// 1. modules folder inside project:
		// root/
		//   main.tf
		//     modules/
		//       ...
		// In this case, if we detect a change in modules/, we will determine
		// the project root to be at root/.
		//
		// 2. shared top-level modules folder
		// root/
		//  project1/
		//    main.tf # uses modules via ../modules
		//  project2/
		//    main.tf # uses modules via ../modules
		//  modules/
		//    ...
		// In this case, if we detect a change in modules/ we don't know which
		// project was using this module so we can't suggest a project root, but we
		// also detect that there's no main.tf in the parent folder of modules/
		// so we won't suggest that as a project. So in this case we return nothing.
		// The code below makes this happen.

		// Need to add a trailing slash before splitting on modules/ because if
		// the input was modules/file.tf then path.Dir will be "modules" and so our
		// split on "modules/" will fail.
		dirWithTrailingSlash := dir + "/"
		modulesSplit := strings.SplitN(dirWithTrailingSlash, "modules/", 2)
		modulesParent := modulesSplit[0]

		// Now we check whether there is a main.tf in the parent.
		if _, err := os.Stat(filepath.Join(repoDir, modulesParent, "main.tf")); os.IsNotExist(err) {
			return ""
		}
		return path.Clean(modulesParent)
	}

	// If it wasn't a modules directory, we assume we're in a project and return
	// this directory.
	return dir
}

// unique de-duplicates strs.
func (p *PushEventRootFinder) unique(strs []string) []string {
	hash := make(map[string]bool)
	var unique []string
	for _, s := range strs {
		if !hash[s] {
			unique = append(unique, s)
			hash[s] = true
		}
	}
	return unique
}

// removeNonExistingDirs removes paths from relativePaths that don't exist.
// relativePaths is a list of paths relative to absRepoDir.
func (p *PushEventRootFinder) removeNonExistingDirs(relativePaths []string, absRepoDir string) []string {
	var filtered []string
	for _, pth := range relativePaths {
		absPath := filepath.Join(absRepoDir, pth)
		if _, err := os.Stat(absPath); !os.IsNotExist(err) {
			filtered = append(filtered, pth)
		}
	}
	return filtered
}
