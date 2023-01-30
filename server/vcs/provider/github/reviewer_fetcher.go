package github

import (
	"context"
	gh "github.com/google/go-github/v45/github"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
)

const ApprovalState = "APPROVED"

type PRReviewFetcher struct {
	ClientCreator githubapp.ClientCreator
}

func (r *PRReviewFetcher) ListReviews(ctx context.Context, installationToken int64, repo models.Repo, prNum int) ([]*gh.PullRequestReview, error) {
	client, err := r.ClientCreator.NewInstallationClient(installationToken)
	if err != nil {
		return nil, errors.Wrap(err, "creating installation client")
	}

	run := func(ctx context.Context, nextPage int) ([]*gh.PullRequestReview, *gh.Response, error) {
		listOptions := gh.ListOptions{
			PerPage: 100,
		}
		listOptions.Page = nextPage
		return client.PullRequests.ListReviews(ctx, repo.Owner, repo.Name, prNum, &listOptions)
	}
	return Iterate(ctx, run)
}
