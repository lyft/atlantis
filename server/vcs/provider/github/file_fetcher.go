package github

import (
	"context"
	gh "github.com/google/go-github/v45/github"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
)

type RemoteFileFetcher struct {
	GithubListIterator *ListIterator
}

type FileFetcherOptions struct {
	Sha   string
	PRNum int
}

func (r *RemoteFileFetcher) GetModifiedFiles(ctx context.Context, repo models.Repo, installationToken int64, fileFetcherOptions FileFetcherOptions) ([]string, error) {
	var run func(ctx context.Context, client *gh.Client, nextPage int) (interface{}, *gh.Response, error)
	if fileFetcherOptions.Sha != "" {
		run = GetCommit(repo, fileFetcherOptions)
	} else if fileFetcherOptions.PRNum != 0 {
		run = ListFiles(repo, fileFetcherOptions)
	} else {
		return nil, errors.New("invalid fileFetcherOptions")
	}
	process := func(i interface{}) []string {
		var files []string
		pageFiles := i.([]gh.CommitFile)
		for _, f := range pageFiles {
			files = append(files, f.GetFilename())

			// If the file was renamed, we'll want to run plan in the directory
			// it was moved from as well.
			if f.GetStatus() == "renamed" {
				files = append(files, f.GetPreviousFilename())
			}
		}
		return files
	}
	return r.GithubListIterator.Iterate(ctx, installationToken, run, process)
}

func GetCommit(repo models.Repo, fileFetcherOptions FileFetcherOptions) func(ctx context.Context, client *gh.Client, nextPage int) (interface{}, *gh.Response, error) {
	return func(ctx context.Context, client *gh.Client, nextPage int) (interface{}, *gh.Response, error) {
		listOptions := gh.ListOptions{
			PerPage: 100,
		}
		listOptions.Page = nextPage
		repositoryCommit, resp, err := client.Repositories.GetCommit(ctx, repo.Owner, repo.Name, fileFetcherOptions.Sha, &listOptions)
		if repositoryCommit != nil {
			return repositoryCommit.Files, resp, err
		}
		return nil, nil, errors.New("unable to retrieve commit files from GH commit")
	}
}

func ListFiles(repo models.Repo, fileFetcherOptions FileFetcherOptions) func(ctx context.Context, client *gh.Client, nextPage int) (interface{}, *gh.Response, error) {
	return func(ctx context.Context, client *gh.Client, nextPage int) (interface{}, *gh.Response, error) {
		listOptions := gh.ListOptions{
			PerPage: 100,
		}
		listOptions.Page = nextPage
		return client.PullRequests.ListFiles(ctx, repo.Owner, repo.Name, fileFetcherOptions.PRNum, &listOptions)
	}
}
