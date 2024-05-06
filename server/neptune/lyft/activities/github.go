package activities

import (
	"context"
	"fmt"
	"net/http"

	"github.com/palantir/go-githubapp/githubapp"

	"github.com/google/go-github/v45/github"
	"github.com/pkg/errors"
	internal "github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
)

type Github struct {
	ClientCreator  githubapp.ClientCreator
	InstallationID int64
}

type ListPRsRequest struct {
	Repo    internal.Repo
	State   internal.PullRequestState
	SortKey internal.SortKey
	Order   internal.Order
}

type ListPRsResponse struct {
	PullRequests []internal.PullRequest
}

func (a *Github) GithubListPRs(ctx context.Context, request ListPRsRequest) (ListPRsResponse, error) {
	prs, err := a.listPullRequests(
		ctx, a.InstallationID,
		request.Repo.Owner,
		request.Repo.Name,
		request.Repo.DefaultBranch,
		string(request.State),
		string(request.SortKey),
		string(request.Order),
	)
	if err != nil {
		return ListPRsResponse{}, errors.Wrap(err, "listing open pull requests")
	}

	pullRequests := []internal.PullRequest{}
	for _, pullRequest := range prs {

		isAutomated := IsPRAutomated(pullRequest)

		pullRequests = append(pullRequests, internal.PullRequest{
			Number:        pullRequest.GetNumber(),
			UpdatedAt:     pullRequest.GetUpdatedAt(),
			IsAutomatedPR: isAutomated,
		})
	}

	return ListPRsResponse{
		PullRequests: pullRequests,
	}, nil
}

func IsPRAutomated(pr *github.PullRequest) bool {
	if pr.Labels == nil {
		return false
	}
	for _, label := range pr.Labels {
		if label.GetName() == "automated" {
			return true
		}
	}
	return false
}

type ListModifiedFilesRequest struct {
	Repo        internal.Repo
	PullRequest internal.PullRequest
}

type ListModifiedFilesResponse struct {
	FilePaths []string
}

func (a *Github) GithubListModifiedFiles(ctx context.Context, request ListModifiedFilesRequest) (ListModifiedFilesResponse, error) {
	files, err := a.listModifiedFiles(
		ctx, a.InstallationID,
		request.Repo.Owner,
		request.Repo.Name,
		request.PullRequest.Number,
	)
	if err != nil {
		return ListModifiedFilesResponse{}, errors.Wrap(err, "listing modified files in pr")
	}

	filepaths := []string{}
	for _, file := range files {
		filepaths = append(filepaths, file.GetFilename())

		// account for previous file name as well if the file has moved across roots
		if file.GetStatus() == "renamed" {
			filepaths = append(filepaths, file.GetPreviousFilename())
		}
	}

	// strings are utf-8 encoded of size 1 to 4 bytes, assuming each file path is of length 100, max size of a filepath = 4 * 100 = 400 bytes
	// upper limit of 2Mb can accomodate (2*1024*1024)/400 = 524k filepaths which is >> max number of results supported by the GH API 3000.
	return ListModifiedFilesResponse{
		FilePaths: filepaths,
	}, nil
}

func (a *Github) listModifiedFiles(ctx context.Context, installationToken int64, owner, repo string, pullNumber int) ([]*github.CommitFile, error) {
	client, err := a.ClientCreator.NewInstallationClient(installationToken)
	if err != nil {
		return nil, errors.Wrap(err, "creating client from installation")
	}

	run := func(ctx context.Context, nextPage int) ([]*github.CommitFile, *github.Response, error) {
		listOptions := github.ListOptions{
			PerPage: 100,
		}
		listOptions.Page = nextPage
		return client.PullRequests.ListFiles(ctx, owner, repo, pullNumber, &listOptions)
	}

	return iterate(ctx, run)
}

func (a *Github) listPullRequests(ctx context.Context, installationToken int64, owner, repo, base, state, sortBy, order string) ([]*github.PullRequest, error) {
	client, err := a.ClientCreator.NewInstallationClient(installationToken)
	if err != nil {
		return nil, errors.Wrap(err, "creating client from installation")
	}

	run := func(ctx context.Context, nextPage int) ([]*github.PullRequest, *github.Response, error) {
		prListOptions := github.PullRequestListOptions{
			State: state,
			Base:  base,
			ListOptions: github.ListOptions{
				PerPage: 100,
			},
			Sort:      sortBy,
			Direction: order,
		}
		prListOptions.ListOptions.Page = nextPage
		return client.PullRequests.List(ctx, owner, repo, &prListOptions)
	}

	return iterate(ctx, run)
}

func iterate[T interface{}](
	ctx context.Context,
	runFunc func(ctx context.Context, nextPage int) ([]T, *github.Response, error)) ([]T, error) {
	var output []T
	nextPage := 0
	for {
		results, resp, err := runFunc(ctx, nextPage)
		if err != nil {
			return nil, errors.Wrap(err, "error running gh api call")
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("not ok status running gh api call: %s", resp.Status)
		}
		output = append(output, results...)
		if resp.NextPage == 0 {
			break
		}
		nextPage = resp.NextPage
	}
	return output, nil
}
