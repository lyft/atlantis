package source

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/logging"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_tmp_working_dir.go TmpWorkingDir

const workingDirPrefix = "repos"

// TmpWorkingDir handles a tmp workspace on disk for running commands.
type TmpWorkingDir interface {
	Clone(baseRepo models.Repo, sha string, destination string) error
	DeleteClone(filePath string) error
	GenerateDirPath(repoName string) string
}

// TmpFileWorkspace
type TmpFileWorkspace struct {
	Credentials vcs.GithubCredentials
	DataDir     string
	// TestingOverrideHeadCloneURL can be used during testing to override the
	// URL of the head repo to be cloned. If it's empty then we clone normally.
	TestingOverrideHeadCloneURL string
	// TestingOverrideBaseCloneURL can be used during testing to override the
	// URL of the base repo to be cloned. If it's empty then we clone normally.
	TestingOverrideBaseCloneURL string
	Logger                      logging.Logger
	GithubHostname              string
}

func (w *TmpFileWorkspace) Clone(baseRepo models.Repo, sha string, destinationPath string) error {
	token, err := w.Credentials.GetToken()
	if err != nil {
		return errors.Wrap(err, "getting github token")
	}

	home, err := homedir.Dir()
	if err != nil {
		return errors.Wrap(err, "getting home dir to write ~/.repo-credentials file")
	}

	// https://developer.github.com/apps/building-github-apps/authenticating-with-github-apps/#http-based-git-access-by-an-installation
	if err := events.WriteGitCreds("x-access-token", token, w.GithubHostname, home, w.Logger, true); err != nil {
		return err
	}

	// Realistically, this is a super brittle way of supporting clones using gh app installation tokens
	// This URL should be built during Repo creation and the struct should be immutable going forward.
	// Doing this requires a larger refactor however, and can probably be coupled with supporting > 1 installation
	authURL := fmt.Sprintf("://x-access-token:%s", token)
	baseRepo.CloneURL = strings.Replace(baseRepo.CloneURL, "://:", authURL, 1)
	baseRepo.SanitizedCloneURL = strings.Replace(baseRepo.SanitizedCloneURL, "://:", "://x-access-token:", 1)

	return w.clone(baseRepo, sha, destinationPath)
}

func (w *TmpFileWorkspace) clone(baseRepo models.Repo, sha string, destinationPath string) error {
	// Create the directory and parents if necessary.
	if err := os.MkdirAll(destinationPath, 0700); err != nil {
		return errors.Wrap(err, "creating new directory")
	}

	// During testing, we mock some of this out.
	baseCloneURL := baseRepo.CloneURL
	if w.TestingOverrideBaseCloneURL != "" {
		baseCloneURL = w.TestingOverrideBaseCloneURL
	}

	// Clone default branch into clone directory
	cloneCmd := []string{"repo", "clone", "--branch", baseRepo.DefaultBranch, "--single-branch", baseCloneURL, destinationPath}
	_, err := w.run(cloneCmd, destinationPath)
	if err != nil {
		return errors.New("failed to clone directory")
	}

	cdCmd := []string{"cd", destinationPath}
	_, err = w.run(cdCmd, destinationPath)
	if err != nil {
		return errors.New("failed to cd into directory")
	}

	checkoutCmd := []string{"repo", "checkout", sha}
	_, err = w.run(checkoutCmd, destinationPath)
	if err != nil {
		return errors.New("failed to clone directory")
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
