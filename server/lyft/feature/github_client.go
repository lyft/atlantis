package feature

import (
	"context"
	"github.com/pkg/errors"
	gh "github.com/runatlantis/atlantis/server/vcs/provider/github"
)

type RepoConfig struct {
	Owner  string
	Repo   string
	Branch string
	Path   string
}

type installationFetcher interface {
	FindOrganizationInstallation(ctx context.Context, org string) (gh.Installation, error)
}

type fileContentsFetcher interface {
	FetchFileContents(ctx context.Context, installationToken int64, owner, repo, branch, path string) ([]byte, error)
}

type CustomGithubInstallationRetriever struct {
	InstallationFetcher installationFetcher
	FileContentsFetcher fileContentsFetcher
	Cfg                 RepoConfig
}

func (c *CustomGithubInstallationRetriever) Retrieve(ctx context.Context) ([]byte, error) {
	installationToken, err := c.InstallationFetcher.FindOrganizationInstallation(ctx, c.Cfg.Owner)
	if err != nil {
		return nil, errors.Wrap(err, "fetching installation token")
	}
	file, err := c.FileContentsFetcher.FetchFileContents(ctx, installationToken.Token, c.Cfg.Owner, c.Cfg.Repo, c.Cfg.Branch, c.Cfg.Path)
	if err != nil {
		return nil, errors.Wrap(err, "fetching file contents")
	}
	return file, nil
}
