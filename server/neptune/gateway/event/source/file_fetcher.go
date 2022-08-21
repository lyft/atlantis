package source

import (
	"context"
	"fmt"
	"github.com/google/go-github/v45/github"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/runatlantis/atlantis/server/events/models"
	"net/http"
)

//go:generate pegomock generate --use-experimental-model-gen --package mocks -o mocks/mock_file_fetcher.go FileFetcher
type FileFetcher interface {
	GetModifiedFilesFromCommit(ctx context.Context, repo models.Repo, sha string, installationToken int64) ([]string, error)
}

type GithubFileFetcher struct {
	ClientCreator githubapp.ClientCreator
}

func (g *GithubFileFetcher) GetModifiedFilesFromCommit(ctx context.Context, repo models.Repo, sha string, installationToken int64) ([]string, error) {
	var files []string
	nextPage := 0
	for {
		opts := github.ListOptions{
			PerPage: 300,
		}
		if nextPage != 0 {
			opts.Page = nextPage
		}
		client, err := g.ClientCreator.NewInstallationClient(installationToken)
		repositoryCommit, resp, err := client.Repositories.GetCommit(ctx, repo.Owner, repo.Name, sha, &opts)
		if err != nil {
			return files, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("error fetching repository contents: %s", resp.Status)
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
