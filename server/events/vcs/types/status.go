package types

import "github.com/runatlantis/atlantis/server/events/models"

type UpdateReqIdentifier struct {
	Repo       models.Repo
	Ref        string
	StatusName string
}
type UpdateStatusRequest struct {
	UpdateReqIdentifier

	// if not present, should be -1
	PullNum     int
	State       models.CommitStatus
	Description string
	DetailsURL  string
}
