package yaml

import (
	"os"
	"path/filepath"

	version "github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/yaml/raw"
	"github.com/runatlantis/atlantis/server/events/yaml/valid"

	"github.com/lyft/datum/pkg/manifest"
)

// ManifestYAMLFilename is the name of the config file for each repo.
const ManifestYAMLFilename = "manifest.yaml"

// ManifestParser find and converts all manifest files that contains
// orchestration.roots
type ManifestParser struct{}

type manifestRoot struct {
	dirPath  string
	manifest *manifest.Manifest
}

// ToRepoCfg converts manifestRoots to valid.repoCfg configuration object.
func (m *ManifestParser) ToRepoCfg(manifests []*manifestRoot) (valid.RepoCfg, error) {
	var repoCfg valid.RepoCfg
	repoCfg.Version = 3

	for _, manifestRoot := range manifests {
		roots := manifestRoot.manifest.Orchestration.Roots.List()
		for _, root := range roots {
			project := valid.Project{}
			projectName := manifestRoot.manifest.Name + "-" + root.Name
			relPath := filepath.Join(manifestRoot.dirPath, root.Directory)
			autoplan := raw.DefaultAutoPlan()

			if len(root.WhenModified) > 0 {
				autoplan.WhenModified = root.WhenModified
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
			project.Workspace = raw.DefaultWorkspace
			project.Autoplan = autoplan

			repoCfg.Projects = append(repoCfg.Projects, project)
		}

	}

	if err := validateProjectNames(repoCfg); err != nil {
		return valid.RepoCfg{}, err
	}

	return repoCfg, nil
}

// FindManifestRoots recursively searches for maniest.yaml file in the
// repository. And only selects the ones with Orchestration.Roots present. All
// eligible roots are wrapped into a struct that contains relPath for the root
// folder and datum.manifest object.
func (m *ManifestParser) FindManifestRoots(repositoryRoot string) ([]*manifestRoot, error) {
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
func (m *ManifestParser) discoverManifests(repositoryRoot string) []string {
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
