package github

import (
	"context"
	gh "github.com/google/go-github/v45/github"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
	"time"
)

type PRCommitFetcher struct {
	ClientCreator githubapp.ClientCreator
}

func (c *PRCommitFetcher) FetchLatestCommitTime(ctx context.Context, installationToken int64, repo models.Repo, prNum int) (time.Time, error) {
	client, err := c.ClientCreator.NewInstallationClient(installationToken)
	if err != nil {
		return time.Time{}, errors.Wrap(err, "creating installation client")
	}
	run := func(ctx context.Context, nextPage int) ([]*gh.RepositoryCommit, *gh.Response, error) {
		listOptions := gh.ListOptions{
			PerPage: 100,
		}
		listOptions.Page = nextPage
		return client.PullRequests.ListCommits(ctx, repo.Owner, repo.Name, prNum, &listOptions)
	}
	commits, err := Iterate(ctx, run)
	if err != nil {
		return time.Time{}, errors.Wrap(err, "iterating through entries")
	}
	latestCommitTimestamp := time.Time{}
	for _, commit := range commits {
		if commit.GetCommit() == nil {
			return time.Time{}, errors.New("getting latest commit")
		}
		if commit.GetCommit().GetCommitter() == nil {
			return time.Time{}, errors.New("getting latest committer")
		}
		commitTimestamp := commit.GetCommit().GetCommitter().GetDate()
		if commitTimestamp.After(latestCommitTimestamp) {
			latestCommitTimestamp = commitTimestamp
		}
	}
	return latestCommitTimestamp, nil
}
