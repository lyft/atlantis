package event

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event/preworkflow"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event/source"
)

type ProjectConfigBuilder struct {
	TmpWorkingDir    source.TmpWorkingDir
	AutoplanFileList string
	PreWorkflowHooks preworkflow.HooksRunner
	ParserValidator  *config.ParserValidator
	ProjectFinder    source.ProjectFinder
	FileFetcher      source.FileFetcher
	GlobalCfg        valid.GlobalCfg
	Logger           logging.Logger
}

//go:generate pegomock generate --use-experimental-model-gen --package mocks -o mocks/mock_project_config_builder.go ProjectBuilder
type ProjectBuilder interface {
	BuildProjectConfigs(ctx context.Context, event Push) ([]*valid.MergedProjectCfg, error)
}

func (b *ProjectConfigBuilder) BuildProjectConfigs(ctx context.Context, event Push) ([]*valid.MergedProjectCfg, error) {
	// Continue if preworkflow hooks fail
	repoDir, err := b.PreWorkflowHooks.Run(event.Repo, event.Sha)
	if err != nil {
		b.Logger.Error(fmt.Sprintf("Error running pre-workflow hooks %s. Proceeding with root building.", err))
	}

	modifiedFiles, err := b.FileFetcher.GetModifiedFilesFromCommit(ctx, event.Repo, event.Sha, event.InstallationToken)
	if err != nil {
		return nil, errors.Wrapf(err, "finding modified files: %s", modifiedFiles)
	}
	// generate a new directory path if pre-workflow hook failed
	if repoDir == "" {
		repoDir = b.TmpWorkingDir.GenerateDirPath(event.Repo.FullName)
	}
	err = b.TmpWorkingDir.Clone(event.Repo, event.Sha, repoDir)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("creating temporary clone at path: %s", repoDir))
	}
	deleteFn := func() {
		if err := b.TmpWorkingDir.DeleteClone(repoDir); err != nil {
			b.Logger.ErrorContext(ctx, "failed deleting cloned repo", map[string]interface{}{
				"err": err,
			})
		}
	}
	defer deleteFn()

	// Parse repo file if it exists.
	hasRepoCfg, err := b.ParserValidator.HasRepoCfg(repoDir)
	if err != nil {
		return nil, errors.Wrapf(err, "looking for %s file in %q", config.AtlantisYAMLFilename, repoDir)
	}

	var mergedProjectCfgs []*valid.MergedProjectCfg
	if hasRepoCfg {
		// If there's a repo cfg then we'll use it to figure out which projects
		// should be planed.
		repoCfg, err := b.ParserValidator.ParseRepoCfg(repoDir, b.GlobalCfg, event.Repo.ID())
		if err != nil {
			return nil, errors.Wrapf(err, "parsing %s", config.AtlantisYAMLFilename)
		}
		matchingProjects, err := b.ProjectFinder.DetermineProjectsViaConfig(modifiedFiles, repoCfg, repoDir)
		if err != nil {
			return nil, err
		}
		for _, mp := range matchingProjects {
			mergedProjectCfg := b.GlobalCfg.MergeProjectCfg(event.Repo.ID(), mp, repoCfg)
			mergedProjectCfgs = append(mergedProjectCfgs, &mergedProjectCfg)
		}
	} else {
		// If there is no repo file, then we'll plan each project that
		// our algorithm determines was modified.
		modifiedProjects := b.ProjectFinder.DetermineProjects(modifiedFiles, event.Repo.FullName, repoDir, b.AutoplanFileList)
		for _, mp := range modifiedProjects {
			mergedProjectCfg := b.GlobalCfg.DefaultProjCfg(event.Repo.ID(), mp.Path, "")
			mergedProjectCfgs = append(mergedProjectCfgs, &mergedProjectCfg)
		}
	}
	if len(mergedProjectCfgs) == 0 {
		return nil, errors.New("event generated 0 project configs")
	}
	return mergedProjectCfgs, nil
}

type MockProjectConfigBuilder struct {
	ProjectCfgs []*valid.MergedProjectCfg
}

func (m *MockProjectConfigBuilder) BuildProjectConfigs(_ context.Context, _ Push) ([]*valid.MergedProjectCfg, error) {
	return m.ProjectCfgs, nil
}
