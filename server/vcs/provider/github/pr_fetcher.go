package github

import (
	"context"
	"fmt"
	gh "github.com/google/go-github/v45/github"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"net/http"
)

type PRFetcher struct {
	ClientCreator githubapp.ClientCreator
}

func (c *PRFetcher) Fetch(ctx context.Context, installationToken int64, repoOwner string, repoName string, prNum int) (*gh.PullRequest, error) {
	client, err := c.ClientCreator.NewInstallationClient(installationToken)
	if err != nil {
		return nil, errors.Wrap(err, "creating installation client")
	}
	pr, resp, err := client.PullRequests.Get(ctx, repoOwner, repoName, prNum)
	if err != nil {
		return nil, errors.Wrap(err, "error running gh api call")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("not ok status running gh api call: %s", resp.Status)
	}
	return pr, nil
}
