package local

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const workingDirPrefix = "repos"

// TmpWorkingDir handles a tmp workspace on disk for running commands.
type TmpWorkingDir interface {
	Clone(baseRepo models.Repo, sha string, destination string) error
	DeleteClone(filePath string) error
	GenerateDirPath(repoName string) string
}

// TmpFileWorkspace
type TmpFileWorkspace struct {
	DataDir string
}

func (w *TmpFileWorkspace) Clone(baseRepo models.Repo, sha string, destinationPath string) error {
	// Create the directory and parents if necessary.
	if err := os.MkdirAll(destinationPath, 0700); err != nil {
		return errors.Wrap(err, "creating new directory")
	}

	// Clone default branch into clone directory
	cloneCmd := []string{"git", "clone", "--branch", baseRepo.DefaultBranch, "--single-branch", baseRepo.CloneURL, destinationPath}
	_, err := w.run(cloneCmd, destinationPath)
	if err != nil {
		return errors.New("failed to clone directory")
	}

	// Return immediately if commit at HEAD of clone matches request commit
	revParseCmd := []string{"git", "rev-parse", "HEAD"}
	revParseOutput, err := w.run(revParseCmd, destinationPath)
	currCommit := strings.Trim(string(revParseOutput), "\n")
	if strings.HasPrefix(currCommit, sha) {
		return nil
	}

	// Otherwise, checkout the correct sha
	checkoutCmd := []string{"git", "checkout", sha}
	_, err = w.run(checkoutCmd, destinationPath)
	if err != nil {
		return errors.New(fmt.Sprintf("failed to checkout to sha: %s", sha))
	}
	return nil
}

func (w *TmpFileWorkspace) DeleteClone(filePath string) error {
	return os.RemoveAll(filePath)
}

func (w *TmpFileWorkspace) GenerateDirPath(repoName string) string {
	return filepath.Join(w.DataDir, workingDirPrefix, repoName, uuid.New().String())
}

func (w *TmpFileWorkspace) run(args []string, destinationPath string) ([]byte, error) {
	cmd := exec.Command(args[0], args[1:]...) // nolint: gosec
	cmd.Dir = destinationPath
	// The repo merge command requires these env vars are set.
	cmd.Env = append(os.Environ(), []string{
		"EMAIL=atlantis@runatlantis.io",
		"GIT_AUTHOR_NAME=atlantis",
		"GIT_COMMITTER_NAME=atlantis",
	}...)
	return cmd.CombinedOutput()
}
