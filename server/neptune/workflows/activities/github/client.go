package github

import (
	"context"
	"net/url"

	"github.com/google/go-github/v45/github"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	gh_helper "github.com/runatlantis/atlantis/server/vcs/provider/github"
)

type Client struct {
	ClientCreator  githubapp.ClientCreator
	InstallationID int64
}

func (c *Client) CreateCheckRun(ctx context.Context, owner, repo string, opts github.CreateCheckRunOptions) (*github.CheckRun, *github.Response, error) {
	client, err := c.ClientCreator.NewInstallationClient(c.InstallationID)

	if err != nil {
		return nil, nil, errors.Wrap(err, "creating client from installation")
	}

	return client.Checks.CreateCheckRun(ctx, owner, repo, opts)
}
func (c *Client) UpdateCheckRun(ctx context.Context, owner, repo string, checkRunID int64, opts github.UpdateCheckRunOptions) (*github.CheckRun, *github.Response, error) {
	client, err := c.ClientCreator.NewInstallationClient(c.InstallationID)

	if err != nil {
		return nil, nil, errors.Wrap(err, "creating client from installation")
	}

	return client.Checks.UpdateCheckRun(ctx, owner, repo, checkRunID, opts)
}
func (c *Client) GetArchiveLink(ctx context.Context, owner, repo string, archiveformat github.ArchiveFormat, opts *github.RepositoryContentGetOptions, followRedirects bool) (*url.URL, *github.Response, error) {
	client, err := c.ClientCreator.NewInstallationClient(c.InstallationID)

	if err != nil {
		return nil, nil, errors.Wrap(err, "creating client from installation")
	}

	return client.Repositories.GetArchiveLink(ctx, owner, repo, archiveformat, opts, followRedirects)
}

func (c *Client) CompareCommits(ctx context.Context, owner, repo string, base, head string, opts *github.ListOptions) (*github.CommitsComparison, *github.Response, error) {
	client, err := c.ClientCreator.NewInstallationClient(c.InstallationID)

	if err != nil {
		return nil, nil, errors.Wrap(err, "creating client from installation")
	}

	return client.Repositories.CompareCommits(ctx, owner, repo, base, head, opts)
}

func (c *Client) ListReviews(ctx context.Context, owner string, repo string, number int) ([]*github.PullRequestReview, error) {
	client, err := c.ClientCreator.NewInstallationClient(c.InstallationID)
	if err != nil {
		return nil, errors.Wrap(err, "creating client from installation")
	}

	run := func(ctx context.Context, nextPage int) ([]*github.PullRequestReview, *github.Response, error) {
		listOptions := github.ListOptions{
			PerPage: 100,
		}
		listOptions.Page = nextPage
		return client.PullRequests.ListReviews(ctx, owner, repo, number, &listOptions)
	}
	return gh_helper.Iterate(ctx, run)
}

func (c *Client) GetPullRequest(ctx context.Context, owner, repo string, number int) (*github.PullRequest, *github.Response, error) {
	client, err := c.ClientCreator.NewInstallationClient(c.InstallationID)
	if err != nil {
		return nil, nil, errors.Wrap(err, "creating client from installation")
	}
	return client.PullRequests.Get(ctx, owner, repo, number)
}

func (c *Client) ListCommits(ctx context.Context, owner string, repo string, number int) ([]*github.RepositoryCommit, error) {
	client, err := c.ClientCreator.NewInstallationClient(c.InstallationID)
	if err != nil {
		return nil, errors.Wrap(err, "creating client from installation")
	}

	run := func(ctx context.Context, nextPage int) ([]*github.RepositoryCommit, *github.Response, error) {
		listOptions := github.ListOptions{
			PerPage: 100,
		}
		listOptions.Page = nextPage
		return client.PullRequests.ListCommits(ctx, owner, repo, number, &listOptions)
	}
	return gh_helper.Iterate(ctx, run)
}

func (c *Client) DismissReview(ctx context.Context, owner string, repo string, number int, reviewID int64, review *github.PullRequestReviewDismissalRequest) (*github.PullRequestReview, *github.Response, error) {
	client, err := c.ClientCreator.NewInstallationClient(c.InstallationID)
	if err != nil {
		return nil, nil, errors.Wrap(err, "creating client from installation")
	}
	return client.PullRequests.DismissReview(ctx, owner, repo, number, reviewID, review)
}

func (c *Client) ListTeamMembers(ctx context.Context, org string, teamSlug string) ([]*github.User, error) {
	client, err := c.ClientCreator.NewInstallationClient(c.InstallationID)
	if err != nil {
		return nil, errors.Wrap(err, "creating client from installation")
	}

	run := func(ctx context.Context, nextPage int) ([]*github.User, *github.Response, error) {
		listOptions := &github.TeamListTeamMembersOptions{
			ListOptions: github.ListOptions{
				PerPage: 100,
			},
		}
		listOptions.Page = nextPage
		return client.Teams.ListTeamMembersBySlug(ctx, org, teamSlug, listOptions)
	}
	return gh_helper.Iterate(ctx, run)
}

func (c *Client) CreateComment(ctx context.Context, owner string, repo string, number int, comment *github.IssueComment) (*github.IssueComment, *github.Response, error) {
	client, err := c.ClientCreator.NewInstallationClient(c.InstallationID)
	if err != nil {
		return nil, nil, err
	}
	return client.Issues.CreateComment(ctx, owner, repo, number, comment)
}
