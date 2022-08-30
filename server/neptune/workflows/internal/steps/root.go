package steps

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

type Job struct {
	Steps []Step
}

// Step was taken from the Atlantis OG config, we might be able to clean this up/remove it
type Step struct {
	StepName  string
	ExtraArgs []string
	// RunCommand is either a custom run step or the command to run
	// during an env step to populate the environment variable dynamically.
	RunCommand string
	// EnvVarName is the name of the
	// environment variable that should be set by this step.
	EnvVarName string
	// EnvVarValue is the value to set EnvVarName to.
	EnvVarValue string
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
