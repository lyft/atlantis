package command

import (
	"time"

	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/uber-go/tally"
)

type executionMode int

const (
	DefaultExecutionMode executionMode = iota
	PullRequestExecutionMode
	DeploymentExecutionMode
)

// CommandTrigger represents the how the command was triggered
type CommandTrigger int

const (
	// Commands that are automatically triggered (ie. automatic plans)
	AutoTrigger CommandTrigger = iota

	// Commands that are triggered by comments (ie. atlantis plan)
	CommentTrigger
)

// Context represents the context of a command that should be executed
// for a pull request.
type Context struct {
	// HeadRepo is the repository that is getting merged into the BaseRepo.
	// If the pull request branch is from the same repository then HeadRepo will
	// be the same as BaseRepo.
	// See https://help.github.com/articles/about-pull-request-merges/.
	HeadRepo models.Repo
	Pull     models.PullRequest
	Scope    tally.Scope
	// User is the user that triggered this command.
	User models.User
	Log  logging.SimpleLogging

	// Current PR state
	PullRequestStatus models.PullReqStatus

	PullStatus *models.PullStatus

	Trigger CommandTrigger

	// Time Atlantis received VCS event, triggering command to be executed
	TriggerTimestamp time.Time

	// ExecutionMode define what mode command is running in. It can be:
	// DefaultExecutionMode - default atlantis behaviour with plan and applying
	// happening within the PR
	// PullRequestExecutionMode - in platform mode PR are only used for plan and policy checks
	// DeploymentExecutionMode - in platform mode deployment mode is post PR merge
	ExecutionMode executionMode
}
