package root

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
)

type DeploymentInfo struct {
	Version    string
	ID         string
	CheckRunID int64
	Revision   string
	User       github.User
	Repo       github.Repo
	Root       Root
	Tags       map[string]string
}
