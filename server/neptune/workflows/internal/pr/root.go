package pr

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/notifier"

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
