package local_test

import (
	"crypto/tls"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/events/vcs/fixtures"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event/local"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

// Test that if we don't have any existing files, we check out the repo with a github app.
func TestClone_GithubAppNoneExisting(t *testing.T) {
	// Initialize the git repo.
	repoDir, cleanupRepo := initRepo(t)
	defer cleanupRepo()
	sha := appendCommit(t, repoDir, ".gitkeep", "initial commit")
	expCommit := runCmd(t, repoDir, "git", "rev-parse", "HEAD")

	dataDir, cleanupDataDir := tempDir(t)
	defer cleanupDataDir()
	wd := &local.TmpFileWorkspace{
		DataDir: dataDir,
	}
	defer disableSSLVerification()()
	testServer, err := fixtures.GithubAppTestServer(t)
	assert.NoError(t, err)
	logger := logging.NewNoopCtxLogger(t)
	gwd := &local.GithubAppWorkingDir{
		TmpWorkingDir: wd,
		Credentials: &vcs.GithubAppCredentials{
			Key:      []byte(fixtures.GithubPrivateKey),
			AppID:    1,
			Hostname: testServer,
		},
		GithubHostname: testServer,
		Logger:         logger,
	}
	destinationPath := wd.GenerateDirPath("nish/repo")
	err = gwd.Clone(newBaseRepo(repoDir), sha, destinationPath)
	assert.NoError(t, err)

	// Use rev-parse to verify at correct commit.
	actCommit := runCmd(t, destinationPath, "git", "rev-parse", "HEAD")
	assert.Equal(t, expCommit, actCommit)
}

func TestClone_GithubAppSetsCorrectUrl(t *testing.T) {
	workingTmpDir := &MockSuccessRepoDir{}
	credentials := &local.GithubAnonymousCredentials{}
	logger := logging.NewNoopCtxLogger(t)

	ghAppWorkingDir := &local.GithubAppWorkingDir{
		TmpWorkingDir:  workingTmpDir,
		Credentials:    credentials,
		GithubHostname: "some-host",
		Logger:         logger,
	}

	baseRepo, _ := models.NewRepo(
		models.Github,
		"runatlantis/atlantis",
		"https://github.com/runatlantis/atlantis.git",
		// user and token have to be blank otherwise this proxy wouldn't be invoked to begin with
		"",
		"",
	)

	modifiedBaseRepo := baseRepo
	modifiedBaseRepo.CloneURL = "https://x-access-token:token@github.com/runatlantis/atlantis.git"
	modifiedBaseRepo.SanitizedCloneURL = "https://x-access-token:<redacted>@github.com/runatlantis/atlantis.git"

	sha := "1234"
	destinationPath := "/path/to/dest"

	err := ghAppWorkingDir.Clone(baseRepo, sha, destinationPath)
	assert.NoError(t, err, "clone url mutation error")
}

// disableSSLVerification disables ssl verification for the global http client
// and returns a function to be called in a defer that will re-enable it.
func disableSSLVerification() func() {
	orig := http.DefaultTransport.(*http.Transport).TLSClientConfig
	// nolint: gosec
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	return func() {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = orig
	}
}

type MockSuccessRepoDir struct {
	DirPath string
}

func (m *MockSuccessRepoDir) Clone(_ models.Repo, _ string, _ string) error {
	return nil
}

func (m *MockSuccessRepoDir) DeleteClone(_ string) error {
	return nil
}

func (m *MockSuccessRepoDir) GenerateDirPath(_ string) string {
	return m.DirPath
}
