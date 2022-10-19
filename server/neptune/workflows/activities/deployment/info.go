package deployment

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
)

const DeploymentInfoVersion = "1.0.0"

type Info struct {
	Version    string
	ID         string
	CheckRunID int64
	Revision   string
	User       github.User
	Repo       github.Repo
	Root       terraform.Root
	Tags       map[string]string
}
