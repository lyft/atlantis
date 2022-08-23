package preworkflow_test

import (
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models/fixtures"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event/preworkflow"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event/source"
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
			WorkingDir:   &source.MockSuccessTmpFileWorkspace{DirPath: repoDir},
			HookExecutor: &preworkflow.MockSuccessPreWorkflowHookExecutor{},
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
			WorkingDir:   &source.MockSuccessTmpFileWorkspace{DirPath: repoDir},
			HookExecutor: &preworkflow.MockSuccessPreWorkflowHookExecutor{},
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
			WorkingDir:   &source.MockFailureTmpFileWorkspace{},
			HookExecutor: &preworkflow.MockSuccessPreWorkflowHookExecutor{},
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
			WorkingDir:   &source.MockSuccessTmpFileWorkspace{DirPath: repoDir},
			HookExecutor: &preworkflow.MockFailurePreWorkflowHookExecutor{},
			GlobalCfg:    globalCfg,
		}
		whDir, err := wh.Run(fixtures.GithubRepo, sha)
		assert.Error(t, err, "error not nil")
		assert.Empty(t, whDir)
	})
}
