package yaml

import (
	"fmt"
	"os"
	"path/filepath"

	version "github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/yaml/valid"

	"github.com/lyft/datum/pkg/manifest"
)

// ManifestYAMLFilename is the name of the config file for each repo.
const ManifestYAMLFilename = "manifest.yaml"

// ManifestFinderConverter find and converts all manifest files that contains
// orchestration.roots
type ManifestFinderConverter struct{}

type manifestRoot struct {
	dirPath  string
	manifest *manifest.Manifest
}

// ToRepoCfg converts manifestRoots to valid.repoCfg configuration object.
func (m *ManifestFinderConverter) ToRepoCfg(manifests []*manifestRoot) (valid.RepoCfg, error) {
	var repoCfg valid.RepoCfg
	repoCfg.Version = 3

	for _, manifestRoot := range manifests {
		roots := manifestRoot.manifest.Orchestration.Roots.List()
		for _, root := range roots {
			project := valid.Project{}
			whenModified := []string{"**/*.tf"}
			projectName := manifestRoot.manifest.Name + "_" + root.Name
			relPath := filepath.Join(manifestRoot.dirPath, root.Directory)

			if len(root.WhenModified) > 0 {
				whenModified = root.WhenModified
			}

			if root.TerraformVersion != "" {
				tfVersion, err := version.NewVersion(root.TerraformVersion)
				if err != nil {
					return valid.RepoCfg{}, errors.Wrapf(err, "can't parse terraform version %s", root.TerraformVersion)
				}

				project.TerraformVersion = tfVersion
			}

			project.Dir = relPath
			project.Name = &projectName
			project.Workspace = root.Name
			project.Autoplan = valid.Autoplan{
				Enabled:      true,
				WhenModified: whenModified,
			}

			repoCfg.Projects = append(repoCfg.Projects, project)
		}

	}

	if err := m.validateProjectNames(repoCfg); err != nil {
		return valid.RepoCfg{}, err
	}

	return repoCfg, nil
}

// FindManifestRoots recursively searches for maniest.yaml file in the
// repository. And only selects the ones with Orchestration.Roots present. All
// eligible roots are wrapped into a struct that contains relPath for the root
// folder and datum.manifest object.
func (m *ManifestFinderConverter) FindManifestRoots(repositoryRoot string) ([]*manifestRoot, error) {
	manifestFiles := m.discoverManifests(repositoryRoot)
	manifestRoots := []*manifestRoot{}

	for _, relPath := range manifestFiles {
		manifestData, err := os.Open(filepath.Join(repositoryRoot, relPath))
		if err != nil {
			return nil, err
		}

		manifestDatum, err := manifest.Load(manifestData)
		if err != nil {
			return nil, errors.Wrapf(err, "unable parse manifest %s", relPath)
		}

		if len(manifestDatum.Orchestration.Roots.List()) > 0 {
			manifestRoot := &manifestRoot{
				dirPath:  filepath.Dir(relPath),
				manifest: manifestDatum,
			}
			manifestRoots = append(manifestRoots, manifestRoot)
		}
	}

	return manifestRoots, nil
}

// Returns a list of relative paths to manifest files in the given directory
func (m *ManifestFinderConverter) discoverManifests(repositoryRoot string) []string {
	var manifests []string

	filepath.Walk(repositoryRoot, func(path string, info os.FileInfo, err error) error {
		if info.Name() == ManifestYAMLFilename && !info.IsDir() {
			// Ignoring this error as it cannot happen, the path is by definition
			// in the root directory
			relativePath, _ := filepath.Rel(repositoryRoot, path)
			manifests = append(manifests, relativePath)
		}

		return nil
	})

	return manifests
}

func (m *ManifestFinderConverter) validateProjectNames(config valid.RepoCfg) error {
	// First, validate that all names are unique.
	seen := make(map[string]bool)
	for _, project := range config.Projects {
		if project.Name != nil {
			name := *project.Name
			exists := seen[name]
			if exists {
				return fmt.Errorf("found two or more projects with name %q; project names must be unique", name)
			}
			seen[name] = true
		}
	}

	// Next, validate that all dir/workspace combos are named.
	// This map's keys will be 'dir/workspace' and the values are the names for
	// that project.
	dirWorkspaceToNames := make(map[string][]string)
	for _, project := range config.Projects {
		key := fmt.Sprintf("%s/%s", project.Dir, project.Workspace)
		names := dirWorkspaceToNames[key]

		// If there is already a project with this dir/workspace then this
		// project must have a name.
		if len(names) > 0 && project.Name == nil {
			return fmt.Errorf("there are two or more projects with dir: %q workspace: %q that are not all named; they must have a 'name' key so they can be targeted for apply's separately", project.Dir, project.Workspace)
		}
		var name string
		if project.Name != nil {
			name = *project.Name
		}
		dirWorkspaceToNames[key] = append(dirWorkspaceToNames[key], name)
	}

	return nil
}
