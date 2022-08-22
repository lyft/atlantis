package deploy

import (
	"github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/steps"
)

// Types defined here should not be used internally, as our goal should be to eventually swap these out for something less brittle than json translation
type Request struct {
	GHRequestID string
	Repository  Repo
	Root        Root
}

type Repo struct {
	// FullName is the owner and repo name separated
	// by a "/"
	FullName string
	// Owner is just the repo owner
	Owner string
	// Name is just the repo name, this will never have
	// /'s in it.
	Name string
	// URL is the ssh clone URL (ie. git@github.com:owner/repo.git)
	URL string

	Credentials AppCredentials
}

type AppCredentials struct {
	InstallationToken int64
}

type Root struct {
	Name  string
	Apply Job
	Plan  Job
}

type Job struct {
	Steps []steps.Step

	// JobContext has all the information needed to run this job
	JobContext JobContext
}

type User struct {
	Username string
}

type Commit struct {
	Ref    string
	Author User
}

type JobContext struct {
	// TerraformVersion is the version of terraform we should use when executing
	// commands for this project. This can be set to nil in which case we will
	// use the default Atlantis terraform version.
	TerraformVersion *version.Version

	// Repo is the repository that the commit is merged into.
	Repo Repo

	Path string

	Commit Commit

	// RepoRelDir is the directory of this project relative to the repo root.
	RepoRelDir string

	// ProjectName is the name of the project set in atlantis.yaml. If there was
	// no name this will be an empty string.
	ProjectName string

	// Workspace is the Terraform workspace this project is in. It will always
	// be set.
	Workspace string
}
