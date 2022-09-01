package event_test

import (
	"context"
	"errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event"
	"github.com/stretchr/testify/assert"
	"testing"
)

var pushEvent event.Push
var rcb event.RootConfigBuilder
var globalCfg valid.GlobalCfg
var expectedErr = errors.New("some error")

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
	// creates a default PCB to used in each test; individual tests mutate a specific field to test certain functionalities
	rcb = event.RootConfigBuilder{
		RepoFetcher:     &mockRepoFetcher{},
		HooksRunner:     &mockHooksRunner{},
		ParserValidator: &mockParserValidator{},
		RootFinder:      &mockRootFinder{},
		FileFetcher:     &mockFileFetcher{},
		GlobalCfg:       globalCfg,
		Logger:          logging.NewNoopCtxLogger(t),
	}
}

func TestRootConfigBuilder_Success(t *testing.T) {
	setupTesting(t)
	projects := []valid.Project{
		{
			Name: &pushEvent.Repo.FullName,
		},
	}
	rcb.RootFinder = &mockRootFinder{
		ConfigProjects: projects,
	}
	projCfg := globalCfg.MergeProjectCfg(pushEvent.Repo.ID(), projects[0], valid.RepoCfg{})
	expProjectConfigs := []*valid.MergedProjectCfg{
		&projCfg,
	}
	projectConfigs, err := rcb.Build(context.Background(), pushEvent)
	assert.NoError(t, err)
	assert.Equal(t, expProjectConfigs, projectConfigs)
}

func TestRootConfigBuilder_DetermineRootsError(t *testing.T) {
	setupTesting(t)
	mockRootFinder := &mockRootFinder{
		error: expectedErr,
	}
	rcb.RootFinder = mockRootFinder
	projectConfigs, err := rcb.Build(context.Background(), pushEvent)
	assert.Error(t, err)
	assert.Empty(t, projectConfigs)

}

func TestRootConfigBuilder_ParserValidatorParseError(t *testing.T) {
	setupTesting(t)
	mockParserValidator := &mockParserValidator{
		error: expectedErr,
	}
	rcb.ParserValidator = mockParserValidator
	projectConfigs, err := rcb.Build(context.Background(), pushEvent)
	assert.Error(t, err)
	assert.Empty(t, projectConfigs)

}

func TestRootConfigBuilder_GetModifiedFilesError(t *testing.T) {
	setupTesting(t)
	rcb.FileFetcher = &mockFileFetcher{
		error: expectedErr,
	}
	projectConfigs, err := rcb.Build(context.Background(), pushEvent)
	assert.Error(t, err)
	assert.Empty(t, projectConfigs)
}

func TestRootConfigBuilder_CloneError(t *testing.T) {
	setupTesting(t)
	rcb.RepoFetcher = &mockRepoFetcher{
		cloneError: expectedErr,
	}
	projectConfigs, err := rcb.Build(context.Background(), pushEvent)
	assert.Error(t, err)
	assert.Empty(t, projectConfigs)

}

func TestRootConfigBuilder_HooksRunnerError(t *testing.T) {
	setupTesting(t)
	mockHooksRunner := &mockHooksRunner{
		error: expectedErr,
	}
	rcb.HooksRunner = mockHooksRunner
	projectConfigs, err := rcb.Build(context.Background(), pushEvent)
	assert.Error(t, err)
	assert.Empty(t, projectConfigs)

}

// Mock implementations

type mockRepoFetcher struct {
	dirPath    string
	cloneError error
}

func (r *mockRepoFetcher) Fetch(_ context.Context, _ models.Repo, _ string) (string, func(ctx context.Context, filePath string), error) {
	return "", func(ctx context.Context, filePath string) {}, r.cloneError
}

type mockHooksRunner struct {
	error error
}

func (h *mockHooksRunner) Run(_ models.Repo, _ string) error {
	return h.error
}

type mockFileFetcher struct {
	error error
}

func (f *mockFileFetcher) GetModifiedFilesFromCommit(_ context.Context, _ models.Repo, _ string, _ int64) ([]string, error) {
	return nil, f.error
}

type mockRootFinder struct {
	ConfigProjects []valid.Project
	error          error
}

func (m *mockRootFinder) FindRoots(_ []string, _ valid.RepoCfg) ([]valid.Project, error) {
	return m.ConfigProjects, m.error
}

type mockParserValidator struct {
	repoCfg valid.RepoCfg
	error   error
}

func (v *mockParserValidator) ParseRepoCfg(_ string, _ string) (valid.RepoCfg, error) {
	return v.repoCfg, v.error
}
