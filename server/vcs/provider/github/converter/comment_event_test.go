package converter

import (
	"testing"

	"github.com/google/go-github/v31/github"
	"github.com/mohae/deepcopy"
	. "github.com/petergtz/pegomock"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/mocks"
	emocks "github.com/runatlantis/atlantis/server/events/mocks"
	"github.com/runatlantis/atlantis/server/events/models"
	. "github.com/runatlantis/atlantis/testing"
)

const (
	GithubUser  = "github-user"
	GithubToken = "github-token"
)

func TestCommentEvent_Convert(t *testing.T) {
	repoConverter := RepoConverter{
		GithubUser:  GithubUser,
		GithubToken: GithubToken,
	}

	eventsParser := events.EventParser{
		GithubUser:  GithubUser,
		GithubToken: GithubToken,
	}
	githubPullGetter := mocks.NewMockGithubPullGetter()
	subject := CommentEventConverter{
		RepoConverter:    repoConverter,
		GithubPullGetter: githubPullGetter,
		EventParser:      &eventsParser,
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

	pull := github.PullRequest{
		User: &github.User{
			Login: github.String("github-user"),
		},
		Head: &github.PullRequestBranch{
			Repo: &Repo,
			Ref:  github.String("aa"),
			SHA:  github.String("1234"),
		},
		HTMLURL: github.String("https://github.com/runatlantis/atlantis/issues/1"),
		Number:  comment.Issue.Number,
		State:   github.String("open"),
		Base: &github.PullRequestBranch{
			Repo: &Repo,
			Ref:  github.String("bb"),
		},
	}

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
		Num:        *pull.Number,
		HeadCommit: *pull.Head.SHA,
		URL:        *pull.HTMLURL,
		HeadBranch: *pull.Head.Ref,
		BaseBranch: *pull.Base.Ref,
		Author:     *pull.User.Login,
		State:      models.OpenPullState,
		BaseRepo:   modelRepo,
	}

	When(githubPullGetter.GetPullRequest(modelRepo, *comment.Issue.Number)).ThenReturn(&pull, nil)
	commentEvent, err := subject.Convert(&comment)
	Ok(t, err)
	Equals(t, modelRepo, commentEvent.BaseRepo)
	Equals(t, models.User{
		Username: *comment.Comment.User.Login,
	}, commentEvent.User)
	Equals(t, *comment.Issue.Number, commentEvent.PullNum)
	Equals(t, modelPull, *commentEvent.Pull)
}

func TestRunCommentCommand_GithubPullErrorf(t *testing.T) {
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

	repoConverter := RepoConverter{
		GithubUser:  GithubUser,
		GithubToken: GithubToken,
	}

	githubPullGetter := mocks.NewMockGithubPullGetter()
	subject := CommentEventConverter{
		RepoConverter:    repoConverter,
		GithubPullGetter: githubPullGetter,
	}
	When(githubPullGetter.GetPullRequest(modelRepo, *comment.Issue.Number)).ThenReturn(nil, errors.New("err"))
	_, err := subject.Convert(&comment)
	ErrContains(t, "making pull request API call to GitHub", err)
}

func TestRunCommentCommand_GithubPullParseErrorf(t *testing.T) {
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

	eventsParser := emocks.NewMockEventParsing()

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

	repoConverter := RepoConverter{
		GithubUser:  GithubUser,
		GithubToken: GithubToken,
	}

	githubPullGetter := mocks.NewMockGithubPullGetter()
	subject := CommentEventConverter{
		RepoConverter:    repoConverter,
		GithubPullGetter: githubPullGetter,
		EventParser:      eventsParser,
	}
	When(githubPullGetter.GetPullRequest(modelRepo, *comment.Issue.Number)).ThenReturn(&github.PullRequest{}, nil)
	When(eventsParser.ParseGithubPull(&github.PullRequest{})).ThenReturn(models.PullRequest{}, models.Repo{}, models.Repo{}, errors.New("error"))
	_, err := subject.Convert(&comment)
	ErrContains(t, "extracting required fields from comment data", err)
}
