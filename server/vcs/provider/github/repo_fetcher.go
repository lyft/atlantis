package github

import (
	"bytes"
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/metrics"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"
	subprocess_exec "github.com/runatlantis/atlantis/server/neptune/exec"
	"github.com/uber-go/tally/v4"
	"os"
	"path/filepath"
	"strings"
)

const workingDirPrefix = "repos"

type tokenGetter interface {
	GetToken() (string, error)
}

// RepoFetcher implements repoFetcher through git clone operations
type RepoFetcher struct {
	DataDir           string
	GithubHostname    string
	Logger            logging.Logger
	GithubCredentials tokenGetter
	GlobalCfg         valid.GlobalCfg
	Scope             tally.Scope
}

type RepoFetcherOptions struct {
	CloneDepth int
}

func (g *RepoFetcher) Fetch(ctx context.Context, repo models.Repo, branch string, sha string, options RepoFetcherOptions) (string, func(ctx context.Context, filePath string), error) {
	home, err := homedir.Dir()
	if err != nil {
		return "", nil, errors.Wrap(err, "getting home dir to write ~/.git-credentials file")
	}

	ghToken, err := g.GithubCredentials.GetToken()
	if err != nil {
		return "", nil, errors.Wrap(err, "fetching github token")
	}

	// https://developer.github.com/apps/building-github-apps/authenticating-with-github-apps/#http-based-git-access-by-an-installation
	if err := WriteGitCreds("x-access-token", ghToken, g.GithubHostname, home, g.Logger, true); err != nil {
		return "", nil, err
	}
	// Realistically, this is a super brittle way of supporting clones using gh app installation tokens
	// This URL should be built during Repo creation and the struct should be immutable going forward.
	// Doing this requires a larger refactor however, and can probably be coupled with supporting > 1 installation
	authURL := fmt.Sprintf("://x-access-token:%s", ghToken)
	repo.CloneURL = strings.Replace(repo.CloneURL, "://:", authURL, 1)
	repo.SanitizedCloneURL = strings.Replace(repo.SanitizedCloneURL, "://:", "://x-access-token:", 1)
	path, cleanup, err := g.clone(ctx, repo, branch, sha, options)
	if err != nil {
		g.Scope.Counter(metrics.ExecutionErrorMetric).Inc(1)
		return path, cleanup, err
	}
	g.Scope.Counter(metrics.ExecutionSuccessMetric).Inc(1)
	return path, cleanup, err
}

func (g *RepoFetcher) clone(ctx context.Context, repo models.Repo, branch string, sha string, options RepoFetcherOptions) (string, func(ctx context.Context, filePath string), error) {
	destinationPath := g.generateDirPath(repo.Name)
	// Create the directory and parents if necessary.
	if err := os.MkdirAll(destinationPath, 0700); err != nil {
		return "", nil, errors.Wrap(err, "creating new directory")
	}

	// Fetch default branch into clone directory
	cloneCmd := []string{"git", "clone", "--branch", branch, "--single-branch", repo.CloneURL, destinationPath}

	if options.CloneDepth > 0 {
		cloneCmd = append(cloneCmd, fmt.Sprintf("--depth=%d", options.CloneDepth))
	}
	_, err := g.run(ctx, cloneCmd, destinationPath)
	if err != nil {
		return "", nil, errors.Wrap(err, "failed to clone directory")
	}

	// Return immediately if commit at HEAD of clone matches request commit
	revParseCmd := []string{"git", "rev-parse", "HEAD"}
	revParseOutput, err := g.run(ctx, revParseCmd, destinationPath)
	if err != nil {
		return "", nil, errors.Wrap(err, "running rev-parse")
	}
	currCommit := strings.Trim(string(revParseOutput), "\n")
	if strings.HasPrefix(currCommit, sha) {
		return destinationPath, g.Cleanup, nil
	}

	// Otherwise, checkout the correct sha
	checkoutCmd := []string{"git", "checkout", sha}
	_, err = g.run(ctx, checkoutCmd, destinationPath)
	if err != nil {
		g.Cleanup(ctx, destinationPath)
		return "", nil, errors.Wrap(err, fmt.Sprintf("failed to checkout to sha: %s", sha))
	}
	return destinationPath, g.Cleanup, nil
}

func (g *RepoFetcher) generateDirPath(repoName string) string {
	return filepath.Join(g.DataDir, workingDirPrefix, repoName, uuid.New().String())
}

func (g *RepoFetcher) run(ctx context.Context, args []string, destinationPath string) ([]byte, error) {
	cmd := subprocess_exec.Command(g.Logger, args[0], args[1:]...) // nolint: gosec
	cmd.Dir = destinationPath
	// The repo merge command requires these env vars are set.
	cmd.Env = append(os.Environ(), []string{
		"EMAIL=atlantis@runatlantis.io",
		"GIT_AUTHOR_NAME=atlantis",
		"GIT_COMMITTER_NAME=atlantis",
	}...)
	var b bytes.Buffer
	cmd.Stdout = &b
	cmd.Stderr = &b
	err := cmd.RunWithNewProcessGroup(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "running command in separate process group")
	}
	return b.Bytes(), nil
}

func (g *RepoFetcher) Cleanup(ctx context.Context, filePath string) {
	if err := os.RemoveAll(filePath); err != nil {
		g.Logger.ErrorContext(ctx, "failed deleting cloned repo", map[string]interface{}{
			"err": err,
		})
	}
}
