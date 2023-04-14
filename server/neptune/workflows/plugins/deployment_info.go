package plugins

import (
	"github.com/google/uuid"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
)

// TerraformDeploymentInfo contains all the information required for a Terraform deployment
// to occur.
type TerraformDeploymentInfo struct {
	ID             uuid.UUID
	Commit         github.Commit
	InitiatingUser github.User
	Root           terraform.Root
	Repo           github.Repo
	Tags           map[string]string
}
