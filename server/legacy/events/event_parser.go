// Copyright 2017 HootSuite Media Inc.
//
// Licensed under the Apache License, Version 2.0 (the License);
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an AS IS BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// Modified hereafter by contributors to runatlantis/atlantis.

package events

import (
	"fmt"
	"path"
	"strings"

	"github.com/google/go-github/v45/github"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/legacy/events/command"
	"github.com/runatlantis/atlantis/server/models"
)

const (
	usagesCols = 90
)

// PullCommand is a command to run on a pull request.
type PullCommand interface {
	// CommandName is the name of the command we're running.
	CommandName() command.Name
	// IsAutoplan is true if this is an autoplan command vs. a comment command.
	IsAutoplan() bool
}

// PolicyCheckCommand is a policy_check command that is automatically triggered
// after successful plan command.
type PolicyCheckCommand struct{}

// CommandName is policy_check.
func (c PolicyCheckCommand) CommandName() command.Name {
	return command.PolicyCheck
}

// IsAutoplan is true for policy_check commands.
func (c PolicyCheckCommand) IsAutoplan() bool {
	return false
}

// AutoplanCommand is a plan command that is automatically triggered when a
// pull request is opened or updated.
type AutoplanCommand struct{}

// CommandName is plan.
func (c AutoplanCommand) CommandName() command.Name {
	return command.Plan
}

// IsAutoplan is true for autoplan commands (obviously).
func (c AutoplanCommand) IsAutoplan() bool {
	return true
}

// CommentCommand is a command that was triggered by a pull request comment.
type CommentCommand struct {
	// RepoRelDir is the path relative to the repo root to run the command in.
	// Will never end in "/". If empty then the comment specified no directory.
	RepoRelDir string
	// Flags are the extra arguments appended to the comment,
	// ex. atlantis plan -- -target=resource
	Flags []string
	// Name is the name of the command the comment specified.
	Name command.Name
	// ForceApply is true of the command should ignore apply_requirments.
	ForceApply bool
	// Workspace is the name of the Terraform workspace to run the command in.
	// If empty then the comment specified no workspace.
	Workspace string
	// ProjectName is the name of a project to run the command on. It refers to a
	// project specified in an atlantis.yaml file.
	// If empty then the comment specified no project.
	ProjectName string
}

// IsForSpecificProject returns true if the command is for a specific dir, workspace
// or project name. Otherwise it's a command like "atlantis plan" or "atlantis
// apply".
func (c CommentCommand) IsForSpecificProject() bool {
	return c.RepoRelDir != "" || c.Workspace != "" || c.ProjectName != ""
}

// CommandName returns the name of this command.
func (c CommentCommand) CommandName() command.Name {
	return c.Name
}

// IsAutoplan will be false for comment commands.
func (c CommentCommand) IsAutoplan() bool {
	return false
}

// String returns a string representation of the command.
func (c CommentCommand) String() string {
	return fmt.Sprintf("command=%q dir=%q workspace=%q project=%q flags=%q", c.Name.String(), c.RepoRelDir, c.Workspace, c.ProjectName, strings.Join(c.Flags, ","))
}

// NewCommentCommand constructs a CommentCommand, setting all missing fields to defaults.
func NewCommentCommand(repoRelDir string, flags []string, name command.Name, forceApply bool, workspace string, project string) *command.Comment {
	// If repoRelDir was empty we want to keep it that way to indicate that it
	// wasn't specified in the comment.
	if repoRelDir != "" {
		repoRelDir = path.Clean(repoRelDir)
		if repoRelDir == "/" {
			repoRelDir = "."
		}
	}
	return &command.Comment{
		RepoRelDir:  repoRelDir,
		Flags:       flags,
		Name:        name,
		Workspace:   workspace,
		ProjectName: project,
		ForceApply:  forceApply,
	}
}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_event_parsing.go EventParsing

// EventParsing parses webhook events from different VCS hosts into their
// respective Atlantis models.
// todo: rename to VCSParsing or the like because this also parses API responses #refactor
//
//nolint:interfacebloat
type EventParsing interface {
	// ParseGithubIssueCommentEvent parses GitHub pull request comment events.
	// baseRepo is the repo that the pull request will be merged into.
	// user is the pull request author.
	// pullNum is the number of the pull request that triggered the webhook.
	// Deprecated: see events/controllers/github/parser.go
	ParseGithubIssueCommentEvent(comment *github.IssueCommentEvent) (
		baseRepo models.Repo, user models.User, pullNum int, err error)

	// ParseGithubPull parses the response from the GitHub API endpoint (not
	// from a webhook) that returns a pull request.
	// pull is the parsed pull request.
	// baseRepo is the repo the pull request will be merged into.
	// headRepo is the repo the pull request branch is from.
	// Deprecated: see converters/github.go
	ParseGithubPull(ghPull *github.PullRequest) (
		pull models.PullRequest, baseRepo models.Repo, headRepo models.Repo, err error)

	// ParseGithubPullEvent parses GitHub pull request events.
	// pull is the parsed pull request.
	// pullEventType is the type of event, for example opened/closed.
	// baseRepo is the repo the pull request will be merged into.
	// headRepo is the repo the pull request branch is from.
	// user is the pull request author.
	// Deprecated: see events/controllers/github/parser.go
	ParseGithubPullEvent(pullEvent *github.PullRequestEvent) (
		pull models.PullRequest, pullEventType models.PullRequestEventType,
		baseRepo models.Repo, headRepo models.Repo, user models.User, err error)

	// ParseGithubRepo parses the response from the GitHub API endpoint that
	// returns a repo into the Atlantis model.
	// Deprecated: see converters/github.go
	ParseGithubRepo(ghRepo *github.Repository) (models.Repo, error)
}

// EventParser parses VCS events.
type EventParser struct {
	GithubUser    string
	GithubToken   string
	AllowDraftPRs bool
}

// ParseGithubIssueCommentEvent parses GitHub pull request comment events.
// See EventParsing for return value docs.
func (e *EventParser) ParseGithubIssueCommentEvent(comment *github.IssueCommentEvent) (baseRepo models.Repo, user models.User, pullNum int, err error) {
	baseRepo, err = e.ParseGithubRepo(comment.Repo)
	if err != nil {
		return
	}
	if comment.Comment == nil || comment.Comment.User.GetLogin() == "" {
		err = errors.New("comment.user.login is null")
		return
	}
	commenterUsername := comment.Comment.User.GetLogin()
	user = models.User{
		Username: commenterUsername,
	}
	pullNum = comment.Issue.GetNumber()
	if pullNum == 0 {
		err = errors.New("issue.number is null")
		return
	}
	return
}

// ParseGithubPullEvent parses GitHub pull request events.
// See EventParsing for return value docs.
func (e *EventParser) ParseGithubPullEvent(pullEvent *github.PullRequestEvent) (pull models.PullRequest, pullEventType models.PullRequestEventType, baseRepo models.Repo, headRepo models.Repo, user models.User, err error) {
	if pullEvent.PullRequest == nil {
		err = errors.New("pull_request is null")
		return
	}
	pull, baseRepo, headRepo, err = e.ParseGithubPull(pullEvent.PullRequest)
	if err != nil {
		return
	}
	if pullEvent.Sender == nil {
		err = errors.New("sender is null")
		return
	}
	senderUsername := pullEvent.Sender.GetLogin()
	if senderUsername == "" {
		err = errors.New("sender.login is null")
		return
	}

	action := pullEvent.GetAction()
	// If it's a draft PR we ignore it for auto-planning if configured to do so
	// however it's still possible for users to run plan on it manually via a
	// comment so if any draft PR is closed we still need to check if we need
	// to delete its locks.
	if pullEvent.GetPullRequest().GetDraft() && pullEvent.GetAction() != "closed" && !e.AllowDraftPRs {
		action = "other"
	}

	switch action {
	case "opened":
		pullEventType = models.OpenedPullEvent
	case "ready_for_review":
		// when an author takes a PR out of 'draft' state a 'ready_for_review'
		// event is triggered. We want atlantis to treat this as a freshly opened PR
		pullEventType = models.OpenedPullEvent
	case "synchronize":
		pullEventType = models.UpdatedPullEvent
	case "closed":
		pullEventType = models.ClosedPullEvent
	default:
		pullEventType = models.OtherPullEvent
	}
	user = models.User{Username: senderUsername}
	return
}

// ParseGithubPull parses the response from the GitHub API endpoint (not
// from a webhook) that returns a pull request.
// See EventParsing for return value docs.
func (e *EventParser) ParseGithubPull(pull *github.PullRequest) (pullModel models.PullRequest, baseRepo models.Repo, headRepo models.Repo, err error) {
	commit := pull.Head.GetSHA()
	if commit == "" {
		err = errors.New("head.sha is null")
		return
	}
	url := pull.GetHTMLURL()
	if url == "" {
		err = errors.New("html_url is null")
		return
	}
	headBranch := pull.Head.GetRef()
	if headBranch == "" {
		err = errors.New("head.ref is null")
		return
	}
	baseBranch := pull.Base.GetRef()
	if baseBranch == "" {
		err = errors.New("base.ref is null")
		return
	}

	authorUsername := pull.User.GetLogin()
	if authorUsername == "" {
		err = errors.New("user.login is null")
		return
	}
	num := pull.GetNumber()
	if num == 0 {
		err = errors.New("number is null")
		return
	}

	baseRepo, err = e.ParseGithubRepo(pull.Base.Repo)
	if err != nil {
		return
	}
	headRepo, err = e.ParseGithubRepo(pull.Head.Repo)
	if err != nil {
		return
	}

	pullState := models.ClosedPullState
	closedAt := pull.GetClosedAt()
	updatedAt := pull.GetUpdatedAt()
	if pull.GetState() == "open" {
		pullState = models.OpenPullState
	}

	pullModel = models.PullRequest{
		Author:     authorUsername,
		HeadBranch: headBranch,
		HeadCommit: commit,
		URL:        url,
		Num:        num,
		State:      pullState,
		BaseRepo:   baseRepo,
		BaseBranch: baseBranch,
		ClosedAt:   closedAt,
		UpdatedAt:  updatedAt,
	}
	return
}

// ParseGithubRepo parses the response from the GitHub API endpoint that
// returns a repo into the Atlantis model.
// See EventParsing for return value docs.
func (e *EventParser) ParseGithubRepo(ghRepo *github.Repository) (models.Repo, error) {
	return models.NewRepo(models.Github, ghRepo.GetFullName(), ghRepo.GetCloneURL(), e.GithubUser, e.GithubToken)
}
