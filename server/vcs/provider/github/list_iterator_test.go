package github_test

import (
	"context"
	gh "github.com/google/go-github/v45/github"
	"github.com/runatlantis/atlantis/server/vcs/provider/github"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

const (
	user1  = "user1"
	user2  = "user2"
	denied = "denied"
)

func TestIterate(t *testing.T) {
	run := func(ctx context.Context, nextPage int) ([]*gh.PullRequestReview, *gh.Response, error) {
		testResults := []*gh.PullRequestReview{
			{
				User:  &gh.User{Login: gh.String(user1)},
				State: gh.String(github.ApprovalState),
			},
			{
				User:  &gh.User{Login: gh.String(user2)},
				State: gh.String(denied),
			},
		}
		testResponse := &gh.Response{
			Response: &http.Response{StatusCode: http.StatusOK},
		}
		return testResults, testResponse, nil
	}
	process := func(reviews []*gh.PullRequestReview) []string {
		var approvalReviewers []string
		for _, review := range reviews {
			if review.GetState() == github.ApprovalState && review.GetUser() != nil {
				approvalReviewers = append(approvalReviewers, review.GetUser().GetLogin())
			}
		}
		return approvalReviewers
	}
	results, err := github.Iterate(context.Background(), run, process)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, results[0], user1)
}

func TestIterate_NotOKStatus(t *testing.T) {
	run := func(ctx context.Context, nextPage int) ([]*gh.PullRequestReview, *gh.Response, error) {
		testResults := []*gh.PullRequestReview{
			{
				User:  &gh.User{Login: gh.String(user1)},
				State: gh.String(github.ApprovalState),
			},
		}
		testResponse := &gh.Response{
			Response: &http.Response{},
		}
		return testResults, testResponse, nil
	}
	process := func(reviews []*gh.PullRequestReview) []string {
		return []string{user1}
	}
	results, err := github.Iterate(context.Background(), run, process)
	assert.Error(t, err)
	assert.Nil(t, results)
}

func TestIterate_ErrorRun(t *testing.T) {
	run := func(ctx context.Context, nextPage int) ([]*gh.PullRequestReview, *gh.Response, error) {
		testResults := []*gh.PullRequestReview{
			{
				User:  &gh.User{Login: gh.String(user1)},
				State: gh.String(github.ApprovalState),
			},
		}
		testResponse := &gh.Response{
			Response: &http.Response{StatusCode: http.StatusOK},
		}
		return testResults, testResponse, assert.AnError
	}
	process := func(reviews []*gh.PullRequestReview) []string {
		return []string{user1}
	}
	results, err := github.Iterate(context.Background(), run, process)
	assert.Error(t, err)
	assert.Nil(t, results)
}
