package models

import (
	"time"

	"github.com/runatlantis/atlantis/server/logging"
	"github.com/uber-go/tally"
)

// CommandTrigger represents the how the command was triggered
type CommandTrigger int

const (
	// Commands that are automatically triggered (ie. automatic plans)
	Auto CommandTrigger = iota

	// Commands that are triggered by comments (ie. atlantis plan)
	Comment
)

// CommandContext represents the context of a command that should be executed
// for a pull request.
type CommandContext struct {
	// HeadRepo is the repository that is getting merged into the BaseRepo.
	// If the pull request branch is from the same repository then HeadRepo will
	// be the same as BaseRepo.
	// See https://help.github.com/articles/about-pull-request-merges/.
	HeadRepo Repo
	Pull     PullRequest
	Scope    tally.Scope
	// User is the user that triggered this command.
	User User
	Log  logging.SimpleLogging

	// Current PR state
	PullRequestStatus PullReqStatus

	PullStatus *PullStatus

	Trigger CommandTrigger

	// Time Atlantis received VCS event, triggering command to be executed
	TriggerTimestamp time.Time
}
