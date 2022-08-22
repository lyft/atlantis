package terraform

import (
	"github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/steps"
)

type Request struct {
	Repo github.Repo
	Root steps.Root

	Commit github.Commit

	// TerraformVersion is the version of terraform we should use when executing
	// commands for this project. This can be set to nil in which case we will
	// use the default Atlantis terraform version.
	TerraformVersion *version.Version

	// ProjectName is the name of the project set in atlantis.yaml. If there was
	// no name this will be an empty string.
	ProjectName string

	// RepoRelDir is the directory of this project relative to the repo root.
	RepoRelDir string

	Path string
}
