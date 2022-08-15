package types

import (
	"time"

	"github.com/runatlantis/atlantis/server/events/models"
)

type UpdateStatusRequest struct {
	Repo        models.Repo
	Ref         string
	State       models.CommitStatus
	StatusName  string
	Description string
	DetailsURL  string
	Output      string
	// if not present, should be -1
	PullNum          int
	PullCreationTime time.Time
	StatusId         string

	// Fields used to support templating for github checks
	CommandName string
	Project     string
	Workspace   string
	Directory   string
}
