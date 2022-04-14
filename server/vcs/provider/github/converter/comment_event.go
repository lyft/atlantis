package converter

import (
	"fmt"
	"time"

	"github.com/google/go-github/v31/github"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/vcs/types/event"
)

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_github_pull_getter.go GithubPullGetter

// GithubPullGetter makes API calls to get pull requests.
type GithubPullGetter interface {
	// GetPullRequest gets the pull request with id pullNum for the repo.
	GetPullRequest(repo models.Repo, pullNum int) (*github.PullRequest, error)
}

type CommentEventConverter struct {
	RepoConverter    RepoConverter
	GithubPullGetter GithubPullGetter
	EventParser      events.EventParsing
}

// Convert converts github issue comment events to our internal representation
// TODO: remove named methods and return CommentEvent directly /techdebt
func (e CommentEventConverter) Convert(comment *github.IssueCommentEvent) (event.Comment, error) {
	baseRepo, err := e.RepoConverter.Convert(comment.Repo)
	if err != nil {
		return event.Comment{}, err
	}
	if comment.Comment == nil || comment.Comment.User.GetLogin() == "" {
		return event.Comment{}, fmt.Errorf("comment.user.login is null")
	}
	commenterUsername := comment.Comment.User.GetLogin()
	user := models.User{
		Username: commenterUsername,
	}
	pullNum := comment.Issue.GetNumber()
	if pullNum == 0 {
		return event.Comment{}, fmt.Errorf("issue.number is null")
	}

	eventTimestamp := time.Now()
	if comment.Comment.CreatedAt != nil {
		eventTimestamp = *comment.Comment.CreatedAt
	}

	// Get the pull req using pull num and base repo
	pull, headRepo, err := e.getGithubData(baseRepo, pullNum)
	if err != nil {
		return event.Comment{}, errors.Wrap(err, "getting pull from github")
	}

	return event.Comment{
		BaseRepo:  baseRepo,
		HeadRepo:  &headRepo,
		Pull:      &pull,
		User:      user,
		PullNum:   pullNum,
		VCSHost:   models.Github,
		Timestamp: eventTimestamp,
		Comment:   comment.GetComment().GetBody(),
	}, nil
}

func (c *CommentEventConverter) getGithubData(baseRepo models.Repo, pullNum int) (models.PullRequest, models.Repo, error) {
	ghPull, err := c.GithubPullGetter.GetPullRequest(baseRepo, pullNum)
	if err != nil {
		return models.PullRequest{}, models.Repo{}, errors.Wrap(err, "making pull request API call to GitHub")
	}
	pull, _, headRepo, err := c.EventParser.ParseGithubPull(ghPull)
	if err != nil {
		return pull, headRepo, errors.Wrap(err, "extracting required fields from comment data")
	}
	return pull, headRepo, nil
}
