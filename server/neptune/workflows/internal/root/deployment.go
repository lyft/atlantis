package root

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
)

type DeploymentInfo struct {
	ID         string
	CheckRunID int64
	Revision   string
	Repo       github.Repo
	Root       Root
}