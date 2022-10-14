package root

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
)

type DeploymentInfo struct {
	Version    string
	ID         string
	CheckRunID int64
	Revision   string
	Repo       github.Repo
	Root       Root
}

func (i *DeploymentInfo) GetRevision() string {
	if i == nil {
		return ""
	}
	return i.Revision
}
