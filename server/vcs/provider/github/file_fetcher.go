package github

import (
	"context"
	"fmt"
	"net/http"

	gh "github.com/google/go-github/v45/github"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
)

type RemoteFileFetcher struct {
	ClientCreator githubapp.ClientCreator
}

type FileFetcherOptions struct {
	Sha   string
	PRNum int
}

func (r *RemoteFileFetcher) GetModifiedFiles(ctx context.Context, repo models.Repo, fileFetcherOptions FileFetcherOptions, installationToken int64) ([]string, error) {
	if fileFetcherOptions.Sha != "" {
		return r.getModifiedFilesFromCommit(ctx, repo, fileFetcherOptions.Sha, installationToken)
	}
	if fileFetcherOptions.PRNum != 0 {
		return r.getModifiedFilesFromPR(ctx, repo, fileFetcherOptions.PRNum, installationToken)
	}
	return nil, errors.New("unable to process FileFetcherOptions")
}

func (r *RemoteFileFetcher) getModifiedFilesFromCommit(ctx context.Context, repo models.Repo, sha string, installationToken int64) ([]string, error) {
	var files []string
	nextPage := 0
	for {
		opts := gh.ListOptions{
			PerPage: 300,
		}
		if nextPage != 0 {
			opts.Page = nextPage
		}
		client, err := r.ClientCreator.NewInstallationClient(installationToken)
		if err != nil {
			return files, errors.Wrap(err, "creating installation client")
		}
		repositoryCommit, resp, err := client.Repositories.GetCommit(ctx, repo.Owner, repo.Name, sha, &opts)
		if err != nil {
			return nil, errors.Wrap(err, "error fetching repository commit")
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("not ok status fetching repository commit: %s", resp.Status)
		}
		for _, f := range repositoryCommit.Files {
			files = append(files, f.GetFilename())

			// If the file was renamed, we'll want to run plan in the directory
			// it was moved from as well.
			if f.GetStatus() == "renamed" {
				files = append(files, f.GetPreviousFilename())
			}
		}
		if resp.NextPage == 0 {
			break
		}
		nextPage = resp.NextPage
	}
	return files, nil
}

// GetModifiedFilesFromPR returns the names of files that were modified in the pull request
// relative to the repo root, e.g. parent/child/file.txt.
func (r *RemoteFileFetcher) getModifiedFilesFromPR(ctx context.Context, repo models.Repo, pullNum int, installationToken int64) ([]string, error) {
	var files []string
	nextPage := 0
	for {
		opts := gh.ListOptions{
			PerPage: 300,
		}
		if nextPage != 0 {
			opts.Page = nextPage
		}
		client, err := r.ClientCreator.NewInstallationClient(installationToken)
		if err != nil {
			return files, errors.Wrap(err, "creating installation client")
		}
		pageFiles, resp, err := client.PullRequests.ListFiles(ctx, repo.Owner, repo.Name, pullNum, &opts)
		if err != nil {
			return files, err
		}
		for _, f := range pageFiles {
			files = append(files, f.GetFilename())

			// If the file was renamed, we'll want to run plan in the directory
			// it was moved from as well.
			if f.GetStatus() == "renamed" {
				files = append(files, f.GetPreviousFilename())
			}
		}
		if resp.NextPage == 0 {
			break
		}
		nextPage = resp.NextPage
	}
	return files, nil
}
