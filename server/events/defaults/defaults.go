package defaults

const (
	// DefaultRepoRelDir is the default directory we run commands in, relative
	// to the root of the repo.
	DefaultRepoRelDir = "."
	// DefaultWorkspace is the default Terraform workspace we run commands in.
	// This is also Terraform's default workspace.
	DefaultWorkspace = "default"
	// DefaultAutomergeEnabled is the default for the automerge setting.
	DefaultAutomergeEnabled = false
	// DefaultParallelApplyEnabled is the default for the parallel apply setting.
	DefaultParallelApplyEnabled = false
	// DefaultParallelPlanEnabled is the default for the parallel plan setting.
	DefaultParallelPlanEnabled = false
)
