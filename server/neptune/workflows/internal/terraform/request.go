package terraform

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/job"
)

type Request struct {
	Root job.Root
	Repo github.RepoInstance
}
