package request

type PlanMode string

const (
	DestroyPlanMode PlanMode = "destroy"
	NormalPlanMode  PlanMode = "normal"
)

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
