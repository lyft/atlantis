package converter

import (
	"testing"

	"github.com/google/go-github/v31/github"
	"github.com/mohae/deepcopy"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
	. "github.com/runatlantis/atlantis/testing"
	"github.com/stretchr/testify/assert"
)

const (
	GithubUser  = "github-user"
	GithubToken = "github-token"
)

type MockPullGetter struct {
	pull *github.PullRequest
	err  error
}

func (p MockPullGetter) GetPullRequest(_ models.Repo, _ int) (*github.PullRequest, error) {
	return p.pull, p.err
}

func setup(t *testing.T) (github.IssueCommentEvent, models.Repo, models.PullRequest, RepoConverter) {
	repoConverter := RepoConverter{
		GithubUser:  GithubUser,
		GithubToken: GithubToken,
	}

	comment := github.IssueCommentEvent{
		Repo: &Repo,
		Issue: &github.Issue{
			Number:  github.Int(1),
			User:    &github.User{Login: github.String("issue_user")},
			HTMLURL: github.String("https://github.com/runatlantis/atlantis/issues/1"),
		},
		Comment: &github.IssueComment{
			User: &github.User{Login: github.String("comment_user")},
		},
	}

	// this should be successful
	modelRepo := models.Repo{
		Owner:             *comment.Repo.Owner.Login,
		FullName:          *comment.Repo.FullName,
		CloneURL:          "https://github-user:github-token@github.com/owner/repo.git",
		SanitizedCloneURL: "https://github-user:<redacted>@github.com/owner/repo.git",
		Name:              "repo",
		VCSHost: models.VCSHost{
			Hostname: "github.com",
			Type:     models.Github,
		},
	}

	modelPull := models.PullRequest{
		Num:        *Pull.Number,
		HeadCommit: *Pull.Head.SHA,
		URL:        *Pull.HTMLURL,
		HeadBranch: *Pull.Head.Ref,
		BaseBranch: *Pull.Base.Ref,
		Author:     *Pull.User.Login,
		State:      models.OpenPullState,
		BaseRepo:   modelRepo,
		UpdatedAt:  *Pull.UpdatedAt,
	}

	return comment, modelRepo, modelPull, repoConverter
}

func TestCommentEvent_Convert_Success(t *testing.T) {
	// setup
	comment, modelRepo, modelPull, repoConverter := setup(t)
	githubPullGetter := MockPullGetter{
		pull: &Pull,
		err:  nil,
	}

	// act
	subject := CommentEventConverter{
		RepoConverter: repoConverter,
		PullGetter:    githubPullGetter,
	}
	commentEvent, err := subject.Convert(&comment)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, modelRepo, commentEvent.BaseRepo)
	assert.Equal(t, models.User{
		Username: *comment.Comment.User.Login,
	}, commentEvent.User)
	assert.Equal(t, *comment.Issue.Number, commentEvent.PullNum)
	assert.Equal(t, modelPull, commentEvent.Pull)
}

func TestCommentEvent_Convert_Fail(t *testing.T) {
	// setup
	comment, _, _, repoConverter := setup(t)
	subject := CommentEventConverter{
		RepoConverter: repoConverter,
	}

	// act and assert
	testComment := deepcopy.Copy(comment).(github.IssueCommentEvent)
	testComment.Comment = nil
	_, err := subject.Convert(&testComment)
	ErrEquals(t, "comment.user.login is null", err)

	testComment = deepcopy.Copy(comment).(github.IssueCommentEvent)
	testComment.Comment.User = nil
	_, err = subject.Convert(&testComment)
	ErrEquals(t, "comment.user.login is null", err)

	testComment = deepcopy.Copy(comment).(github.IssueCommentEvent)
	testComment.Comment.User.Login = nil
	_, err = subject.Convert(&testComment)
	ErrEquals(t, "comment.user.login is null", err)

	testComment = deepcopy.Copy(comment).(github.IssueCommentEvent)
	testComment.Issue = nil
	_, err = subject.Convert(&testComment)
	ErrEquals(t, "issue.number is null", err)
}

func TestRunCommentCommand_GithubPullError(t *testing.T) {
	// setup
	comment, _, _, repoConverter := setup(t)
	githubPullGetter := MockPullGetter{
		pull: nil,
		err:  errors.New("err"),
	}

	// act
	subject := CommentEventConverter{
		RepoConverter: repoConverter,
		PullGetter:    githubPullGetter,
	}
	_, err := subject.Convert(&comment)

	// assert
	ErrContains(t, "getting pull from github", err)
}

func TestRunCommentCommand_GithubPullParseError(t *testing.T) {
	// setup
	comment, _, _, repoConverter := setup(t)
	githubPullGetter := MockPullGetter{
		pull: &github.PullRequest{},
		err:  nil,
	}

	// act
	subject := CommentEventConverter{
		RepoConverter: repoConverter,
		PullGetter:    githubPullGetter,
	}
	_, err := subject.Convert(&comment)

	// assert
	ErrContains(t, "converting pull request type", err)
}
