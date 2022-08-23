package event_test

import (
	"context"
	cfgParser "github.com/runatlantis/atlantis/server/core/config"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event/preworkflow"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event/source"
	"github.com/stretchr/testify/assert"
	"testing"
)

var pushEvent event.Push
var pcb event.ProjectConfigBuilder
var globalCfg valid.GlobalCfg

func setupTesting(t *testing.T) {
	globalCfg = valid.NewGlobalCfg()
	repo := models.Repo{
		FullName:      "nish/repo",
		DefaultBranch: "",
	}
	pushEvent = event.Push{
		Repo: repo,
		Sha:  "1234",
	}
	// creates a default PCB to avoid duplication across each test; each field is mocked out/and modified depending on test constraints
	pcb = event.ProjectConfigBuilder{
		TmpWorkingDir:    &source.MockSuccessTmpFileWorkspace{},
		AutoplanFileList: "",
		PreWorkflowHooks: &preworkflow.MockSuccessPreWorkflowHooksRunner{},
		ParserValidator:  &cfgParser.ParserValidator{},
		ProjectFinder:    &source.MockProjectFinder{},
		FileFetcher:      &source.MockSuccessFileFetcher{},
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
	pcb.ProjectFinder = &source.MockProjectFinder{
		Projects: projects,
	}
	projCfg := globalCfg.DefaultProjCfg(pushEvent.Repo.ID(), "", "")
	expProjectConfigs := []*valid.MergedProjectCfg{
		&projCfg,
	}
	projectConfigs, err := pcb.BuildProjectConfigs(context.Background(), pushEvent)
	assert.NoError(t, err)
	assert.Equal(t, expProjectConfigs, projectConfigs)
}

func TestProjectConfigBuilder_NoProjects(t *testing.T) {
	setupTesting(t)
	projectConfigs, err := pcb.BuildProjectConfigs(context.Background(), pushEvent)
	assert.Error(t, err)
	assert.Empty(t, projectConfigs)
}

func TestProjectConfigBuilder_CloneError(t *testing.T) {
	setupTesting(t)
	pcb.TmpWorkingDir = &source.MockFailureTmpFileWorkspace{}
	projectConfigs, err := pcb.BuildProjectConfigs(context.Background(), pushEvent)
	assert.Error(t, err)
	assert.Empty(t, projectConfigs)
}

func TestProjectConfigBuilder_GetModifiedFilesError(t *testing.T) {
	setupTesting(t)
	pcb.FileFetcher = &source.MockFailureFileFetcher{}
	projectConfigs, err := pcb.BuildProjectConfigs(context.Background(), pushEvent)
	assert.Error(t, err)
	assert.Empty(t, projectConfigs)
}
