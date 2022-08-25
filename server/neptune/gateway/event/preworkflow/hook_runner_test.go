package preworkflow_test

import (
	"errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/models/fixtures"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event/local"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event/preworkflow"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRunPreHooks_Clone(t *testing.T) {
	sha := "1234"
	testHook := &valid.PreWorkflowHook{
		StepName:   "test",
		RunCommand: "some command",
	}
	repoDir := "path/to/repo"

	t.Run("success hooks in cfg", func(t *testing.T) {
		globalCfg := valid.GlobalCfg{
			Repos: []valid.Repo{
				{
					ID: fixtures.GithubRepo.ID(),
					PreWorkflowHooks: []*valid.PreWorkflowHook{
						testHook,
					},
				},
			},
		}
		wh := preworkflow.PreWorkflowHooksRunner{
			RepoDir:      &local.MockSuccessRepoDir{DirPath: repoDir},
			HookExecutor: &MockSuccessPreWorkflowHookExecutor{},
			GlobalCfg:    globalCfg,
		}
		whDir, err := wh.Run(fixtures.GithubRepo, sha)
		assert.Equal(t, whDir, repoDir)
		assert.NoError(t, err)
	})
	t.Run("success hooks not in cfg", func(t *testing.T) {
		globalCfg := valid.GlobalCfg{
			Repos: []valid.Repo{
				// one with hooks but mismatched id
				{
					ID: "id1",
					PreWorkflowHooks: []*valid.PreWorkflowHook{
						testHook,
					},
				},
				// one with the correct id but no hooks
				{
					ID:               fixtures.GithubRepo.ID(),
					PreWorkflowHooks: []*valid.PreWorkflowHook{},
				},
			},
		}
		wh := preworkflow.PreWorkflowHooksRunner{
			RepoDir:      &local.MockSuccessRepoDir{DirPath: repoDir},
			HookExecutor: &MockSuccessPreWorkflowHookExecutor{},
			GlobalCfg:    globalCfg,
		}
		whDir, err := wh.Run(fixtures.GithubRepo, sha)
		assert.NoError(t, err)
		assert.Equal(t, whDir, "")
	})
	t.Run("error cloning", func(t *testing.T) {
		globalCfg := valid.GlobalCfg{
			Repos: []valid.Repo{
				{
					ID: fixtures.GithubRepo.ID(),
					PreWorkflowHooks: []*valid.PreWorkflowHook{
						testHook,
					},
				},
			},
		}
		wh := preworkflow.PreWorkflowHooksRunner{
			RepoDir:      &local.MockFailureRepoDir{},
			HookExecutor: &MockSuccessPreWorkflowHookExecutor{},
			GlobalCfg:    globalCfg,
		}
		whDir, err := wh.Run(fixtures.GithubRepo, sha)
		assert.Error(t, err, "error not nil")
		assert.Empty(t, whDir)
	})
	t.Run("error running pre hook", func(t *testing.T) {
		globalCfg := valid.GlobalCfg{
			Repos: []valid.Repo{
				{
					ID: fixtures.GithubRepo.ID(),
					PreWorkflowHooks: []*valid.PreWorkflowHook{
						testHook,
					},
				},
			},
		}
		wh := preworkflow.PreWorkflowHooksRunner{
			RepoDir:      &local.MockSuccessRepoDir{DirPath: repoDir},
			HookExecutor: &MockFailurePreWorkflowHookExecutor{},
			GlobalCfg:    globalCfg,
		}
		whDir, err := wh.Run(fixtures.GithubRepo, sha)
		assert.Error(t, err, "error not nil")
		assert.Empty(t, whDir)
	})
}

type MockSuccessPreWorkflowHookExecutor struct {
}

func (m *MockSuccessPreWorkflowHookExecutor) Execute(_ *valid.PreWorkflowHook, _ models.Repo, _ string) error {
	return nil
}

type MockFailurePreWorkflowHookExecutor struct {
}

func (m *MockFailurePreWorkflowHookExecutor) Execute(_ *valid.PreWorkflowHook, _ models.Repo, _ string) error {
	return errors.New("some error")
}
