package source_test

import (
	"fmt"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event/source"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// Clone from provided branch
func TestCloneSimple(t *testing.T) {
	// Initialize the git repo.
	repoDir, cleanupRepo := initRepo(t)
	defer cleanupRepo()
	sha := appendCommit(t, repoDir, ".gitkeep", "initial commit")
	expCommit := runCmd(t, repoDir, "git", "rev-parse", "HEAD")

	dataDir, cleanupDataDir := tempDir(t)
	defer cleanupDataDir()
	wd := &source.TmpFileWorkspace{
		DataDir: dataDir,
	}
	destinationPath := wd.GenerateDirPath("nish/repo")
	err := wd.Clone(newBaseRepo(repoDir), sha, destinationPath)
	assert.NoError(t, err)

	// Use rev-parse to verify at correct commit.
	actCommit := runCmd(t, destinationPath, "git", "rev-parse", "HEAD")
	assert.Equal(t, expCommit, actCommit)
}

// Clone with checkout to correct sha needed
func TestCloneCheckout(t *testing.T) {
	// Initialize the git repo.
	repoDir, cleanupRepo := initRepo(t)
	defer cleanupRepo()

	// Add first commit
	sha1 := appendCommit(t, repoDir, ".gitkeep", "initial commit")
	expCommit := runCmd(t, repoDir, "git", "rev-parse", "HEAD")

	// Add second commit
	_ = appendCommit(t, repoDir, ".gitignore", "second commit")

	dataDir, cleanupDataDir := tempDir(t)
	defer cleanupDataDir()
	wd := &source.TmpFileWorkspace{
		DataDir: dataDir,
	}
	destinationPath := wd.GenerateDirPath("nish/repo")
	err := wd.Clone(newBaseRepo(repoDir), sha1, destinationPath)
	assert.NoError(t, err)

	// Use rev-parse to verify at correct commit.
	actCommit := runCmd(t, destinationPath, "git", "rev-parse", "HEAD")
	assert.Equal(t, expCommit, actCommit)
}

// Validate if clone is not possible, we return a failure
func TestSimpleCloneFailure(t *testing.T) {
	// Initialize the git repo.
	repoDir, cleanupRepo := initRepo(t)
	defer cleanupRepo()
	sha := appendCommit(t, repoDir, ".gitkeep", "initial commit")

	dataDir, cleanupDataDir := tempDir(t)
	defer cleanupDataDir()
	wd := &source.TmpFileWorkspace{
		DataDir: dataDir,
	}
	destinationPath := wd.GenerateDirPath("nish/repo")
	repo := newBaseRepo(repoDir)
	repo.DefaultBranch = "invalid-branch"
	err := wd.Clone(repo, sha, destinationPath)
	assert.Error(t, err)
}

// Validate if we do not find requested sha, we don't checkout
func TestCloneCheckoutFailure(t *testing.T) {
	// Initialize the git repo.
	repoDir, cleanupRepo := initRepo(t)
	defer cleanupRepo()

	// Add first commit
	_ = appendCommit(t, repoDir, ".gitkeep", "initial commit")

	// Add second commit
	_ = appendCommit(t, repoDir, ".gitignore", "second commit")

	dataDir, cleanupDataDir := tempDir(t)
	defer cleanupDataDir()
	wd := &source.TmpFileWorkspace{
		DataDir: dataDir,
	}
	destinationPath := wd.GenerateDirPath("nish/repo")
	err := wd.Clone(newBaseRepo(repoDir), "invalidsha", destinationPath)
	assert.Error(t, err)
}

func newBaseRepo(repoDir string) models.Repo {
	return models.Repo{
		VCSHost: models.VCSHost{
			Hostname: "github.com",
		},
		FullName:      "nish/repo",
		DefaultBranch: "branch",
		CloneURL:      fmt.Sprintf("file://%s", repoDir),
	}
}

func initRepo(t *testing.T) (string, func()) {
	repoDir, cleanup := tempDir(t)
	runCmd(t, repoDir, "git", "init")
	return repoDir, cleanup
}

func appendCommit(t *testing.T, repoDir string, fileName string, commitMessage string) string {
	runCmd(t, repoDir, "touch", fileName)
	runCmd(t, repoDir, "git", "add", fileName)
	runCmd(t, repoDir, "git", "config", "--local", "user.email", "atlantisbot@runatlantis.io")
	runCmd(t, repoDir, "git", "config", "--local", "user.name", "atlantisbot")
	output := runCmd(t, repoDir, "git", "commit", "-m", commitMessage)
	// hacky regex to fetch commit sha
	re := regexp.MustCompile("\\w+]")
	commitResult := re.FindString(output)
	sha := commitResult[:len(commitResult)-1]
	runCmd(t, repoDir, "git", "branch", "branch", "-f")
	return sha
}

func runCmd(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cpCmd := exec.Command(name, args...)
	cpCmd.Dir = dir
	cpOut, err := cpCmd.CombinedOutput()
	assert.NoError(t, err, "err running %q: %s", strings.Join(append([]string{name}, args...), " "), cpOut)
	return string(cpOut)
}

func tempDir(t *testing.T) (string, func()) {
	tmpDir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	return tmpDir, func() {
		os.RemoveAll(tmpDir) // nolint: errcheck
	}
}
