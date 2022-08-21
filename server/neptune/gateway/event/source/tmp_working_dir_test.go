package source_test

import (
	"fmt"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event/source"
	. "github.com/runatlantis/atlantis/testing"
	"os/exec"
	"strings"
	"testing"
)

// Test that if we don't have any existing files, we check out the repo.
func TestClone_NoneExisting(t *testing.T) {
	// Initialize the repo repo.
	repoDir, cleanup := initRepo(t)
	defer cleanup()
	expCommit := runCmd(t, repoDir, "repo", "rev-parse", "HEAD")

	dataDir, cleanup2 := TempDir(t)
	defer cleanup2()

	wd := &source.TmpFileWorkspace{
		DataDir:                     dataDir,
		TestingOverrideHeadCloneURL: fmt.Sprintf("file://%s", repoDir),
	}
	destinationPath := wd.GenerateDirPath("nish/repo")
	err := wd.Clone(models.Repo{}, "1234", destinationPath)
	Ok(t, err)

	// Use rev-parse to verify at correct commit.
	actCommit := runCmd(t, destinationPath, "repo", "rev-parse", "HEAD")
	Equals(t, expCommit, actCommit)
}

func initRepo(t *testing.T) (string, func()) {
	repoDir, cleanup := TempDir(t)
	runCmd(t, repoDir, "repo", "init")
	runCmd(t, repoDir, "touch", ".gitkeep")
	runCmd(t, repoDir, "repo", "add", ".gitkeep")
	runCmd(t, repoDir, "repo", "repo", "--local", "user.email", "atlantisbot@runatlantis.io")
	runCmd(t, repoDir, "repo", "repo", "--local", "user.name", "atlantisbot")
	runCmd(t, repoDir, "repo", "commit", "-m", "initial commit")
	runCmd(t, repoDir, "repo", "branch", "branch")
	return repoDir, cleanup
}

func runCmd(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cpCmd := exec.Command(name, args...)
	cpCmd.Dir = dir
	cpOut, err := cpCmd.CombinedOutput()
	Assert(t, err == nil, "err running %q: %s", strings.Join(append([]string{name}, args...), " "), cpOut)
	return string(cpOut)
}
