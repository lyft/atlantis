package event_test

import (
	"context"
	"errors"
	. "github.com/petergtz/pegomock"
	cfgParser "github.com/runatlantis/atlantis/server/core/config"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event"
	preworkflow_mocks "github.com/runatlantis/atlantis/server/neptune/gateway/event/preworkflow/mocks"
	source_mocks "github.com/runatlantis/atlantis/server/neptune/gateway/event/source/mocks"
	"github.com/stretchr/testify/assert"
	"testing"
)

var pushEvent event.Push
var pcb event.ProjectConfigBuilder
var globalCfg valid.GlobalCfg
var projectFinder *source_mocks.MockProjectFinder
var tmpWorkingDir *source_mocks.MockTmpWorkingDir
var fileFetcher *source_mocks.MockFileFetcher

func setupTesting(t *testing.T) {
	RegisterMockTestingT(t)
	globalCfg = valid.NewGlobalCfg()
	repo := models.Repo{
		FullName:      "nish/repo",
		DefaultBranch: "",
	}
	pushEvent = event.Push{
		Repo: repo,
		Sha:  "1234",
	}
	projectFinder = source_mocks.NewMockProjectFinder()
	tmpWorkingDir = source_mocks.NewMockTmpWorkingDir()
	fileFetcher = source_mocks.NewMockFileFetcher()
	pcb = event.ProjectConfigBuilder{
		TmpWorkingDir:    tmpWorkingDir,
		AutoplanFileList: "",
		PreWorkflowHooks: preworkflow_mocks.NewMockHooksRunner(),
		ParserValidator:  &cfgParser.ParserValidator{},
		ProjectFinder:    projectFinder,
		FileFetcher:      fileFetcher,
		GlobalCfg:        globalCfg,
		Logger:           logging.NewNoopCtxLogger(t),
	}
}

func TestProjectConfigBuilder_Success(t *testing.T) {
	setupTesting(t)
	projects := []models.Project{
		{
			RepoFullName: pushEvent.Repo.FullName,
		},
	}
	When(projectFinder.DetermineProjects(nil, pushEvent.Repo.FullName, "", "")).
		ThenReturn(projects)
	projCfg := globalCfg.DefaultProjCfg(pushEvent.Repo.ID(), "", "")
	expProjectConfigs := []*valid.MergedProjectCfg{
		&projCfg,
	}
	projectConfigs, err := pcb.BuildProjectConfigs(context.Background(), pushEvent)
	assert.NoError(t, err)
	assert.Equal(t, expProjectConfigs, projectConfigs)
	projectFinder.VerifyWasCalledOnce().DetermineProjects(nil, pushEvent.Repo.FullName, "", "")
	tmpWorkingDir.VerifyWasCalledOnce().DeleteClone("")
}

func TestProjectConfigBuilder_NoProjects(t *testing.T) {
	setupTesting(t)
	When(projectFinder.DetermineProjects(nil, pushEvent.Repo.FullName, "", "")).
		ThenReturn([]models.Project{})
	projectConfigs, err := pcb.BuildProjectConfigs(context.Background(), pushEvent)
	assert.Error(t, err)
	assert.Empty(t, projectConfigs)
	projectFinder.VerifyWasCalledOnce().DetermineProjects(nil, pushEvent.Repo.FullName, "", "")
	tmpWorkingDir.VerifyWasCalledOnce().DeleteClone("")
}

func TestProjectConfigBuilder_CloneError(t *testing.T) {
	setupTesting(t)
	When(tmpWorkingDir.Clone(pushEvent.Repo, pushEvent.Sha, "")).
		ThenReturn(errors.New("some error"))
	projectConfigs, err := pcb.BuildProjectConfigs(context.Background(), pushEvent)
	assert.Error(t, err)
	assert.Empty(t, projectConfigs)
	tmpWorkingDir.VerifyWasCalled(Never()).DeleteClone("")
	tmpWorkingDir.VerifyWasCalledOnce().Clone(pushEvent.Repo, pushEvent.Sha, "")
}

func TestProjectConfigBuilder_GetModifiedFilesError(t *testing.T) {
	setupTesting(t)
	When(fileFetcher.GetModifiedFilesFromCommit(context.Background(), pushEvent.Repo, pushEvent.Sha, pushEvent.InstallationToken)).
		ThenReturn(nil, errors.New("some error"))
	projectConfigs, err := pcb.BuildProjectConfigs(context.Background(), pushEvent)
	assert.Error(t, err)
	assert.Empty(t, projectConfigs)
	tmpWorkingDir.VerifyWasCalled(Never()).Clone(pushEvent.Repo, pushEvent.Sha, "")
}
