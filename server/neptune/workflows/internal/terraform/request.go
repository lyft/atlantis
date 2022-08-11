package terraform

type Request struct {
	Repo Repo
	Root Root
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
}

type Root struct {
	Name  string
	Apply Job
	Plan  Job
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
