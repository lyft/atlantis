package terraform

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/notifier"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/deployment"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/plugins"

	"github.com/google/uuid"
)

type DeploymentInfo struct {
	ID             uuid.UUID
	CheckRunID     int64
	Commit         github.Commit
	InitiatingUser github.User
	Root           terraform.Root
	Repo           github.Repo
	Tags           map[string]string
}

func (i DeploymentInfo) ToExternalInfo() plugins.TerraformDeploymentInfo {
	return plugins.TerraformDeploymentInfo{
		ID:             i.ID,
		Commit:         i.Commit,
		InitiatingUser: i.InitiatingUser,
		Root:           i.Root,
		Repo:           i.Repo,
		Tags:           i.Tags,
	}
}

func (i DeploymentInfo) BuildPersistableInfo() *deployment.Info {
	return &deployment.Info{
		Version:  deployment.InfoSchemaVersion,
		ID:       i.ID.String(),
		Revision: i.Commit.Revision,
		Branch:   i.Commit.Branch,
		Root: deployment.Root{
			Name:    i.Root.Name,
			Trigger: string(i.Root.Trigger),
		},
		Repo: deployment.Repo{
			Name:  i.Repo.Name,
			Owner: i.Repo.Owner,
		},
	}
}

func (i DeploymentInfo) ToInternalInfo() notifier.Info {
	return notifier.Info{
		ID:       i.ID,
		Commit:   i.Commit,
		RootName: i.Root.Name,
		Repo:     i.Repo,
	}
}
