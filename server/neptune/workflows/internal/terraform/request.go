package terraform

import "github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy"

type Request struct {
	// TODO: potentially separate identical request definition from deploy pkg
	Repo deploy.Repo
	Root deploy.Root
}
