package converter_test

import (
	"context"
	"github.com/runatlantis/atlantis/server/vcs"
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/google/go-github/v45/github"
	"github.com/mohae/deepcopy"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/vcs/provider/github/converter"
	. "github.com/runatlantis/atlantis/testing"
)

var PullEvent = github.PullRequestEvent{
	Sender: &github.User{
		Login: github.String("user"),
	},
	Repo:        &Repo,
	PullRequest: &Pull,
	Action:      github.String("opened"),
	Installation: &github.Installation{
		ID: github.Int64(1),
	},
}

func TestConvert_PullRequestEvent(t *testing.T) {
	repoConverter := converter.RepoConverter{
		GithubUser:  "github-user",
		GithubToken: "github-token",
	}
	subject := converter.PullEventConverter{
		PullConverter: converter.PullConverter{RepoConverter: repoConverter},
		PullStateFetcher: &testPullStateFetcher{
			pull: PullEvent.PullRequest,
		},
	}
	_, err := subject.Convert(context.Background(), &github.PullRequestEvent{})
	ErrEquals(t, "pull_request is null", err)

	testEvent := deepcopy.Copy(PullEvent).(github.PullRequestEvent)
	testEvent.PullRequest.HTMLURL = nil
	_, err = subject.Convert(context.Background(), &testEvent)
	ErrEquals(t, "html_url is null", err)

	actPull, err := subject.Convert(context.Background(), &PullEvent)
	Ok(t, err)
	expBaseRepo := models.Repo{
		Owner:             "owner",
		FullName:          "owner/repo",
		CloneURL:          "https://github-user:github-token@github.com/owner/repo.git",
		SanitizedCloneURL: "https://github-user:<redacted>@github.com/owner/repo.git",
		Name:              "repo",
		VCSHost: models.VCSHost{
			Hostname: "github.com",
			Type:     models.Github,
		},
		DefaultBranch: "main",
	}
	Equals(t, models.PullRequest{
		URL:        Pull.GetHTMLURL(),
		Author:     Pull.User.GetLogin(),
		HeadBranch: Pull.Head.GetRef(),
		BaseBranch: Pull.Base.GetRef(),
		HeadCommit: Pull.Head.GetSHA(),
		Num:        Pull.GetNumber(),
		State:      models.OpenPullState,
		BaseRepo:   expBaseRepo,
		HeadRepo:   expBaseRepo,
		UpdatedAt:  timestamp,
		HeadRef: vcs.Ref{
			Type: "branch",
			Name: "ref",
		},
	}, actPull.Pull)
	Equals(t, models.OpenedPullEvent, actPull.EventType)
	Equals(t, models.User{Username: "user"}, actPull.User)
	Equals(t, int64(1), actPull.InstallationToken)
}

func TestConvert_PullRequestEvent_Draft(t *testing.T) {
	repoConverter := converter.RepoConverter{
		GithubUser:  "github-user",
		GithubToken: "github-token",
	}
	// verify that draft PRs are treated as 'other' events by default
	testEvent := deepcopy.Copy(PullEvent).(github.PullRequestEvent)
	draftPR := true
	testEvent.PullRequest.Draft = &draftPR
	subject := converter.PullEventConverter{
		PullConverter: converter.PullConverter{RepoConverter: repoConverter},
		PullStateFetcher: &testPullStateFetcher{
			pull: testEvent.PullRequest,
		},
	}
	pull, err := subject.Convert(context.Background(), &testEvent)
	Ok(t, err)
	Equals(t, models.OtherPullEvent, pull.EventType)
	// verify that drafts are planned if requested
	subject.AllowDraftPRs = true
	defer func() { subject.AllowDraftPRs = false }()
	pull, err = subject.Convert(context.Background(), &testEvent)
	Ok(t, err)
	Equals(t, models.OpenedPullEvent, pull.EventType)
	Equals(t, int64(1), pull.InstallationToken)
}

func TestConvert_PullRequestEvent_FetchError(t *testing.T) {
	repoConverter := converter.RepoConverter{
		GithubUser:  "github-user",
		GithubToken: "github-token",
	}
	subject := converter.PullEventConverter{
		PullConverter: converter.PullConverter{RepoConverter: repoConverter},
		PullStateFetcher: &testPullStateFetcher{
			err: assert.AnError,
		},
	}
	_, err := subject.Convert(context.Background(), &PullEvent)
	assert.Error(t, err)
}

type testPullStateFetcher struct {
	err  error
	pull *github.PullRequest
}

func (t testPullStateFetcher) FetchLatestState(_ context.Context, _ int64, _ string, _ string, _ int) (*github.PullRequest, error) {
	return t.pull, t.err
}
