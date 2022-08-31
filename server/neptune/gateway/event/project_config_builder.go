package event

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"
)

// repoFetcher manages a cloned repo's workspace on disk for running commands.
type repoFetcher interface {
	Fetch(ctx context.Context, baseRepo models.Repo, sha string) (string, func(ctx context.Context, filePath string), error)
}

// hooksRunner runs preworkflow hooks for a given repository/commit
type hooksRunner interface {
	Run(repo models.Repo, repoDir string) error
}

// fileFetcher handles being able to identify and fetch the changed files per individual commit
type fileFetcher interface {
	GetModifiedFilesFromCommit(ctx context.Context, repo models.Repo, sha string, installationToken int64) ([]string, error)
}

// rootFinder determines which roots were modified in a given event.
type rootFinder interface {
	// DetermineRoots returns the list of roots that were modified
	// based on modifiedFiles and the repo's config.
	DetermineRoots(modifiedFiles []string, config valid.RepoCfg) ([]valid.Project, error)
}

// parserValidator config builds repo specific configurations
type parserValidator interface {
	HasRepoCfg(absRepoDir string) (bool, error)
	ParseRepoCfg(absRepoDir string, globalCfg valid.GlobalCfg, repoID string) (valid.RepoCfg, error)
}

type RootConfigBuilder struct {
	RepoFetcher     repoFetcher
	HooksRunner     hooksRunner
	ParserValidator parserValidator
	RootFinder      rootFinder
	FileFetcher     fileFetcher
	GlobalCfg       valid.GlobalCfg
	Logger          logging.Logger
}

func (b *RootConfigBuilder) Build(ctx context.Context, event Push) ([]*valid.MergedProjectCfg, error) {
	// Generate a new filepath location and clone repo into it
	repoDir, deleteFn, err := b.RepoFetcher.Fetch(ctx, event.Repo, event.Sha)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("creating temporary clone at path: %s", repoDir))
	}
	defer deleteFn(ctx, repoDir)

	// Run pre-workflow hooks
	err = b.HooksRunner.Run(event.Repo, repoDir)
	if err != nil {
		return nil, errors.Wrap(err, "running pre-workflow hooks")
	}

	// Fetch files modified in commit
	modifiedFiles, err := b.FileFetcher.GetModifiedFilesFromCommit(ctx, event.Repo, event.Sha, event.InstallationToken)
	if err != nil {
		return nil, errors.Wrapf(err, "finding modified files: %s", modifiedFiles)
	}

	// Validate rep cfg file exists
	hasRepoCfg, err := b.ParserValidator.HasRepoCfg(repoDir)
	if err != nil {
		return nil, errors.Wrapf(err, "looking for %s file in %q", config.AtlantisYAMLFilename, repoDir)
	}
	if !hasRepoCfg {
		return nil, errors.New("repo cfg file does not exist")
	}

	// Parse repo configs into specific root configs (i.e. roots)
	// TODO: rename project to roots
	var mergedRootCfgs []*valid.MergedProjectCfg
	repoCfg, err := b.ParserValidator.ParseRepoCfg(repoDir, b.GlobalCfg, event.Repo.ID())
	if err != nil {
		return nil, errors.Wrapf(err, "parsing %s", config.AtlantisYAMLFilename)
	}
	matchingRoots, err := b.RootFinder.DetermineRoots(modifiedFiles, repoCfg)
	if err != nil {
		return nil, err
	}
	for _, mr := range matchingRoots {
		mergedRootCfg := b.GlobalCfg.MergeProjectCfg(event.Repo.ID(), mr, repoCfg)
		mergedRootCfgs = append(mergedRootCfgs, &mergedRootCfg)
	}
	if len(mergedRootCfgs) == 0 {
		return nil, errors.New("event generated 0 root configs")
	}
	return mergedRootCfgs, nil
}
