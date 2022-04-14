package converter

import (
	"fmt"
	"time"

	"github.com/google/go-github/v31/github"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/vcs/types/event"
)

// PullGetter makes API calls to get pull requests.
type PullGetter interface {
	// GetPullRequest gets the pull request with id pullNum for the repo.
	GetPullRequest(repo models.Repo, pullNum int) (*github.PullRequest, error)
}

type CommentEventConverter struct {
	RepoConverter RepoConverter
	PullGetter    PullGetter
}

// Convert converts github issue comment events to our internal representation
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

	ghPull, err := e.PullGetter.GetPullRequest(baseRepo, pullNum)
	if err != nil {
		return event.Comment{}, errors.Wrap(err, "getting pull from github")
	}

	pull, _, headRepo, err := e.ParseGithubPull(ghPull)
	if err != nil {
		return event.Comment{}, errors.Wrap(err, "converting pull request type")
	}

	return event.Comment{
		BaseRepo:  baseRepo,
		HeadRepo:  headRepo,
		Pull:      pull,
		User:      user,
		PullNum:   pullNum,
		VCSHost:   models.Github,
		Timestamp: eventTimestamp,
		Comment:   comment.GetComment().GetBody(),
	}, nil
}

// ParseGithubPull parses the response from the GitHub API endpoint (not
// from a webhook) that returns a pull request.
func (e CommentEventConverter) ParseGithubPull(pull *github.PullRequest) (pullModel models.PullRequest, baseRepo models.Repo, headRepo models.Repo, err error) {
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
func (c *CommentEventConverter) ParseGithubRepo(ghRepo *github.Repository) (models.Repo, error) {
	return models.NewRepo(models.Github, ghRepo.GetFullName(), ghRepo.GetCloneURL(), c.RepoConverter.GithubUser, c.RepoConverter.GithubToken)
}
