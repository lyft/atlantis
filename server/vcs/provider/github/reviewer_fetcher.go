package github

import (
	"context"
	gh "github.com/google/go-github/v45/github"
	"github.com/runatlantis/atlantis/server/events/models"
)

const ApprovalState = "APPROVED"

type PRReviewerFetcher struct {
	GithubListIterator *ListIterator
}

func (r *PRReviewerFetcher) ListApprovalReviewers(ctx context.Context, installationToken int64, repo models.Repo, prNum int) ([]string, error) {
	run := func(ctx context.Context, client *gh.Client, nextPage int) (interface{}, *gh.Response, error) {
		listOptions := gh.ListOptions{
			PerPage: 100,
		}
		listOptions.Page = nextPage

		return client.PullRequests.ListReviews(ctx, repo.Owner, repo.Name, prNum, &listOptions)
	}
	process := func(i interface{}) []string {
		var approvalReviewers []string
		reviews := i.([]gh.PullRequestReview)
		for _, review := range reviews {
			if review.GetState() == ApprovalState && review.GetUser() != nil {
				approvalReviewers = append(approvalReviewers, review.GetUser().GetLogin())
			}
		}
		return approvalReviewers
	}
	return r.GithubListIterator.Iterate(ctx, installationToken, run, process)
}
