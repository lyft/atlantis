package steps

import (
	"fmt"
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
	Name string
	Path string
	Repo github.RepoInstance
	Root Root
}

func (r *RootInstance) RelativePathFromRepo() (string, error) {
	return filepath.Rel(r.Path, r.Repo.Path)
}

func (r *RootInstance) GetPlanFilename() string {
	return fmt.Sprintf("%s.tfplan", r.Name)
}

func (r *RootInstance) GetShowResultFileName() string {
	return fmt.Sprintf("%s.json", r.Name)
}

func BuildRootInstanceFrom(root Root, repo github.RepoInstance) *RootInstance {
	return &RootInstance{
		Name: root.Name,
		Path: root.Path,
		Root: root,
		Repo: repo,
	}
}
