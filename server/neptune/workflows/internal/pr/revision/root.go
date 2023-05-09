package revision

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/notifier"
	"github.com/runatlantis/atlantis/server/neptune/workflows/plugins"

	"github.com/google/uuid"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
)

type RootInfo struct {
	ID     uuid.UUID
	Commit github.Commit
	Root   terraform.Root
	Repo   github.Repo
	Tags   map[string]string
}

func (i RootInfo) ToInternalInfo() notifier.Info {
	return notifier.Info{
		ID:       i.ID,
		Commit:   i.Commit,
		RootName: i.Root.Name,
		Repo:     i.Repo,
	}
}

func (i RootInfo) ToExternalInfo() plugins.TerraformDeploymentInfo {
	//TODO: revisit and define separate external info when we need it,
	// currently external notifiers aren't needed for PR mode so we don't know
	// yet what information is needed
	return plugins.TerraformDeploymentInfo{
		ID:     i.ID,
		Commit: i.Commit,
		Root:   i.Root,
		Repo:   i.Repo,
		Tags:   i.Tags,
	}
}
