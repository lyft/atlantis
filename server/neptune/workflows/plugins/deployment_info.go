package plugins

import (
	"github.com/google/uuid"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
)

// ideally we should make this protos or something that has backwards compatible changes
type TerraformDeploymentInfo struct {
	ID             uuid.UUID
	CheckRunID     int64
	Commit         github.Commit
	InitiatingUser github.User
	Root           terraform.Root
	Repo           github.Repo
	Tags           map[string]string
}
