package types

import "github.com/runatlantis/atlantis/server/events/models"

type UpdateStatusRequest struct {
	Repo models.Repo
	// if not present, should be -1
	PullNum     int
	Ref         string
	State       models.CommitStatus
	StatusName  string
	Description string
	DetailsURL  string
	Output      string

	// [WENGINES-4643] TODO: Remove UseGithubChecks flag when github checks is stable.
	// UseGithubChecks is false by default. It is set to true if the output updater determines that there's no existing atlantis status
	// Used to avoid duplicate API call to determine if github checks needs to be used in checks client wrapper
	UseGithubChecks bool
}
