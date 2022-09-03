package root

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/job"
)

// Root is the definition of a root
type Root struct {
	Name string

	// Path is the relative path from the repo
	Path      string
	TfVersion string
	Apply     job.Job
	Plan      job.Job
}

// LocalRoot is a root that exists locally on disk
type LocalRoot struct {
	Root Root
	// Path on disk
	Path string
	Repo github.Repo
}

func (r *LocalRoot) RelativePathFromRepo() string {
	return r.Root.Path
}

func BuildLocalRoot(root Root, repo github.Repo, path string) *LocalRoot {
	return &LocalRoot{
		Root: root,
		Repo: repo,
		Path: path,
	}
}