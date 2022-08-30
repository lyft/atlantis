package job

import (
	"path/filepath"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
)

type Root struct {
	Name      string
	Path      string
	TfVersion string
	Apply     Job
	Plan      Job
}

// Root Instance is a root at a certain commit with the repo info
type RootInstance struct {
	Root Root
	Repo github.RepoInstance
}

func (r *RootInstance) RelativePathFromRepo() (string, error) {
	return filepath.Rel(r.Root.Path, r.Repo.Path)
}

func BuildRootInstanceFrom(root Root, repo github.RepoInstance) *RootInstance {
	return &RootInstance{
		Root: root,
		Repo: repo,
	}
}
