package activities

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/go-github/v45/github"
	"github.com/pkg/errors"
	internal "github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/stretchr/testify/assert"
	sdktemporal "go.temporal.io/sdk/temporal"
)

// testGithubClient embeds the githubClient interface so tests only need to implement the
// methods they exercise; any unimplemented method call will panic via a nil pointer deref.
type testGithubClient struct {
	githubClient
	compareCommitsErr error
	getPullRequestErr error
}

func (c testGithubClient) CompareCommits(ctx context.Context, owner, repo string, base, head string, opts *github.ListOptions) (*github.CommitsComparison, *github.Response, error) {
	return nil, nil, c.compareCommitsErr
}

func (c testGithubClient) GetPullRequest(ctx context.Context, owner, repo string, number int) (*github.PullRequest, *github.Response, error) {
	return nil, nil, c.getPullRequestErr
}

func fakeRequest() *http.Request {
	return &http.Request{Method: http.MethodGet, URL: &url.URL{Path: "/repos/lyft/example"}}
}

func githubNotFoundErr() error {
	return &github.ErrorResponse{
		Response: &http.Response{StatusCode: http.StatusNotFound, Request: fakeRequest()},
		Message:  "Not Found",
	}
}

func githubServerErr() error {
	return &github.ErrorResponse{
		Response: &http.Response{StatusCode: http.StatusInternalServerError, Request: fakeRequest()},
		Message:  "Internal Server Error",
	}
}

func TestGithubCompareCommit_NotFound_ReturnsNonRetryableError(t *testing.T) {
	a := &githubActivities{Client: testGithubClient{compareCommitsErr: githubNotFoundErr()}}

	_, err := a.GithubCompareCommit(context.Background(), CompareCommitRequest{})

	var appErr *sdktemporal.ApplicationError
	assert.True(t, errors.As(err, &appErr), "expected a *temporal.ApplicationError, got %T", err)
	assert.True(t, appErr.NonRetryable())
	assert.Equal(t, GithubResourceNotFoundErrorType, appErr.Type())
}

func TestGithubCompareCommit_OtherError_RemainsRetryable(t *testing.T) {
	a := &githubActivities{Client: testGithubClient{compareCommitsErr: githubServerErr()}}

	_, err := a.GithubCompareCommit(context.Background(), CompareCommitRequest{})

	var appErr *sdktemporal.ApplicationError
	assert.False(t, errors.As(err, &appErr), "did not expect a non-retryable ApplicationError, got %T", err)
	assert.Error(t, err)
}

func TestGithubGetPullRequestState_NotFound_ReturnsNonRetryableError(t *testing.T) {
	a := &githubActivities{Client: testGithubClient{getPullRequestErr: githubNotFoundErr()}}

	_, err := a.GithubGetPullRequestState(context.Background(), GetPullRequestStateRequest{Repo: internal.Repo{}, PRNumber: 1})

	var appErr *sdktemporal.ApplicationError
	assert.True(t, errors.As(err, &appErr), "expected a *temporal.ApplicationError, got %T", err)
	assert.True(t, appErr.NonRetryable())
	assert.Equal(t, GithubResourceNotFoundErrorType, appErr.Type())
}

func TestGithubGetPullRequestState_OtherError_RemainsRetryable(t *testing.T) {
	a := &githubActivities{Client: testGithubClient{getPullRequestErr: githubServerErr()}}

	_, err := a.GithubGetPullRequestState(context.Background(), GetPullRequestStateRequest{Repo: internal.Repo{}, PRNumber: 1})

	var appErr *sdktemporal.ApplicationError
	assert.False(t, errors.As(err, &appErr), "did not expect a non-retryable ApplicationError, got %T", err)
	assert.Error(t, err)
}
