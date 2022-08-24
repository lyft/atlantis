package terraform

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	steps "github.com/runatlantis/atlantis/server/neptune/workflows/internal/root"
)

/*
Root
	Name string
	Path string
	Tf Version string

	Plan Job
		Steps[]
			StepName  string
			ExtraArgs []string
			RunCommand string
			EnvVarName string
			EnvVarValue string
	Apply Job
		Steps[]
			StepName  string
			ExtraArgs []string
			RunCommand string
			EnvVarName string
			EnvVarValue string

Repo
	Owner string
	Name string
	HeadCommit
		Ref string
		Author string
	Path string


RootInstance
	Name string
	Path string
	Repo *RepoInstance
		HeadCommit github.Commit
		Repo github.Repo
		Path string

JobRunner.Run(workflowCtx, job, rootInstance)

StepRunner.Run(workflowCtx, step, rootInstance, executionContext)
*/

type Request struct {
	Root steps.Root
	Repo github.RepoInstance
}
