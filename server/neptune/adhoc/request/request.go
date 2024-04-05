package request

type PlanMode string

const (
	DestroyPlanMode PlanMode = "destroy"
	NormalPlanMode  PlanMode = "normal"
)

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
	// Flag to determine if open PRs for a root need to be rebased
	RebaseEnabled bool
	// Repo's default branch
	DefaultBranch string

	Credentials AppCredentials
}

type AppCredentials struct {
	InstallationToken int64
}

type Root struct {
	Name         string
	Apply        Job
	Plan         Job
	RepoRelPath  string
	TrackedFiles []string
	TfVersion    string
	PlanMode     PlanMode
}

type Job struct {
	Steps []Step
}

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
