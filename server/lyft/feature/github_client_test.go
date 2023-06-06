package feature_test

import (
	"context"
	"github.com/runatlantis/atlantis/server/lyft/feature"
	gh "github.com/runatlantis/atlantis/server/vcs/provider/github"
	"github.com/stretchr/testify/assert"
	"testing"
)

const (
	owner  = "owner"
	repo   = "repo"
	branch = "branch"
	path   = "path"
)

func TestCustomGithubInstallationRetriever_Retrieve(t *testing.T) {
	installationFetcher := &testInstallationFetcher{
		token: 123,
	}
	fileFetcher := &testFileFetcher{
		contents:       []byte("contents"),
		expectedOwner:  owner,
		expectedRepo:   repo,
		expectedBranch: branch,
		expectedPath:   path,
		t:              t,
	}
	repoCfg := feature.RepoConfig{
		Owner:  owner,
		Repo:   repo,
		Branch: branch,
		Path:   path,
	}
	retriever := feature.CustomGithubInstallationRetriever{
		InstallationFetcher: installationFetcher,
		FileContentsFetcher: fileFetcher,
		Cfg:                 repoCfg,
	}
	bytes, err := retriever.Retrieve(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, []byte("contents"), bytes)
}

func TestCustomGithubInstallationRetriever_FileError(t *testing.T) {
	installationFetcher := &testInstallationFetcher{
		token: 123,
	}
	fileFetcher := &testFileFetcher{
		contents:       []byte("contents"),
		expectedOwner:  owner,
		expectedRepo:   repo,
		expectedBranch: branch,
		expectedPath:   path,
		t:              t,
		error:          assert.AnError,
	}
	repoCfg := feature.RepoConfig{
		Owner:  owner,
		Repo:   repo,
		Branch: branch,
		Path:   path,
	}
	retriever := feature.CustomGithubInstallationRetriever{
		InstallationFetcher: installationFetcher,
		FileContentsFetcher: fileFetcher,
		Cfg:                 repoCfg,
	}
	bytes, err := retriever.Retrieve(context.Background())
	assert.Error(t, err)
	assert.Nil(t, bytes)
}

func TestCustomGithubInstallationRetriever_RepoError(t *testing.T) {
	installationFetcher := &testInstallationFetcher{
		token: 123,
		error: assert.AnError,
	}
	fileFetcher := &testFileFetcher{
		contents:       []byte("contents"),
		expectedOwner:  owner,
		expectedRepo:   repo,
		expectedBranch: branch,
		expectedPath:   path,
		t:              t,
	}
	repoCfg := feature.RepoConfig{
		Owner:  owner,
		Repo:   repo,
		Branch: branch,
		Path:   path,
	}
	retriever := feature.CustomGithubInstallationRetriever{
		InstallationFetcher: installationFetcher,
		FileContentsFetcher: fileFetcher,
		Cfg:                 repoCfg,
	}
	bytes, err := retriever.Retrieve(context.Background())
	assert.Error(t, err)
	assert.Nil(t, bytes)
}

type testInstallationFetcher struct {
	token int64
	error error
}

func (t testInstallationFetcher) FindOrganizationInstallation(ctx context.Context, org string) (gh.Installation, error) {
	return gh.Installation{
		Token: t.token,
	}, t.error
}

type testFileFetcher struct {
	contents       []byte
	error          error
	expectedOwner  string
	expectedRepo   string
	expectedBranch string
	expectedPath   string
	t              *testing.T
}

func (f testFileFetcher) FetchFileContents(ctx context.Context, installationToken int64, owner, repo, branch, path string) ([]byte, error) {
	assert.Equal(f.t, owner, f.expectedOwner)
	assert.Equal(f.t, repo, f.expectedRepo)
	assert.Equal(f.t, branch, f.expectedBranch)
	assert.Equal(f.t, path, f.expectedPath)
	return f.contents, f.error
}
