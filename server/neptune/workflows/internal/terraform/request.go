package terraform

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/steps"
)

type Request struct {
	Root steps.Root
	Repo github.RepoInstance
}
