package feature

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/google/go-github/v45/github"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"net/http"
)

type githubClient interface {
	GetContents(owner, repo, branch, path string) ([]byte, error)
}

type RepoConfig struct {
	Owner  string
	Repo   string
	Branch string
	Path   string
}

// CustomGithubRetriever uses Atlantis' internal client to retrieve the contents
// of the feature file.  This allows us to re-use GH credentials easily as opposed
// to the default ffclient.GithubRetriever.
type CustomGithubRetriever struct {
	Client githubClient
	Cfg    RepoConfig
}

func (c *CustomGithubRetriever) Retrieve(ctx context.Context) ([]byte, error) {
	return c.Client.GetContents(c.Cfg.Owner, c.Cfg.Repo, c.Cfg.Branch, c.Cfg.Path)
}

type installationFetcher interface {
	FetchInstallationToken(ctx context.Context) (int64, error)
}

type InstallationFetcher struct {
	ClientCreator githubapp.ClientCreator
	Org           string
}

func (i *InstallationFetcher) FetchInstallationToken(ctx context.Context) (int64, error) {
	appClient, err := i.ClientCreator.NewAppClient()
	if err != nil {
		return 0, errors.Wrap(err, "creating app client")
	}

	installation, _, err := appClient.Apps.FindOrganizationInstallation(ctx, i.Org)
	if err != nil {
		return 0, errors.Wrapf(err, "finding organization installation")
	}
	return installation.GetID(), nil
}

type fileContentsFetcher interface {
	FetchFileContents(ctx context.Context, installationToken int64, owner, repo, branch, path string) ([]byte, error)
}

type FileContentsFetcher struct {
	ClientCreator githubapp.ClientCreator
}

func (f *FileContentsFetcher) FetchFileContents(ctx context.Context, installationToken int64, owner, repo, branch, path string) ([]byte, error) {
	installationClient, err := f.ClientCreator.NewInstallationClient(installationToken)
	if err != nil {
		return nil, errors.Wrap(err, "creating installation client")
	}
	opt := &github.RepositoryContentGetOptions{Ref: branch}
	fileContent, _, resp, err := installationClient.Repositories.GetContents(ctx, owner, repo, path, opt)
	if err != nil {
		return nil, errors.Wrapf(err, "getting repository")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got status code %d", resp.StatusCode)
	}

	decoded, err := base64.StdEncoding.DecodeString(*fileContent.Content)
	if err != nil {
		return nil, errors.Wrapf(err, "decoding file content")
	}

	return decoded, nil
}

type CustomGithubInstallationRetriever struct {
	InstallationFetcher installationFetcher
	FileContentsFetcher fileContentsFetcher
	Cfg                 RepoConfig
}

func (c *CustomGithubInstallationRetriever) Retrieve(ctx context.Context) ([]byte, error) {
	installationToken, err := c.InstallationFetcher.FetchInstallationToken(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "fetching installation token")
	}
	file, err := c.FileContentsFetcher.FetchFileContents(ctx, installationToken, c.Cfg.Owner, c.Cfg.Repo, c.Cfg.Branch, c.Cfg.Path)
	if err != nil {
		return nil, errors.Wrap(err, "fetching file contents")
	}
	return file, nil
}
