package preworkflow_test

import (
	"errors"
	. "github.com/petergtz/pegomock"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models/fixtures"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event/preworkflow"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event/preworkflow/mocks"
	source_mocks "github.com/runatlantis/atlantis/server/neptune/gateway/event/source/mocks"
	"github.com/stretchr/testify/assert"
	"testing"
)

var wh preworkflow.PreWorkflowHooksRunner
var whWorkingDir *source_mocks.MockTmpWorkingDir
var whExecutor *mocks.MockHookExecutor

func preWorkflowHooksSetup(t *testing.T) {
	RegisterMockTestingT(t)
	whWorkingDir = source_mocks.NewMockTmpWorkingDir()
	whExecutor = mocks.NewMockHookExecutor()

	wh = preworkflow.PreWorkflowHooksRunner{
		WorkingDir:   whWorkingDir,
		HookExecutor: whExecutor,
	}
}

func TestRunPreHooks_Clone(t *testing.T) {
	sha := "1234"
	testHook := &valid.PreWorkflowHook{
		StepName:   "test",
		RunCommand: "some command",
	}
	repoDir := "path/to/repo"

	t.Run("success hooks in cfg", func(t *testing.T) {
		preWorkflowHooksSetup(t)
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
		wh.GlobalCfg = globalCfg
		When(whWorkingDir.GenerateDirPath(fixtures.GithubRepo.FullName)).ThenReturn(repoDir)
		When(whWorkingDir.Clone(fixtures.GithubRepo, sha, repoDir)).ThenReturn(nil)
		When(whExecutor.Execute(testHook, fixtures.GithubRepo, repoDir)).ThenReturn(nil)
		whDir, err := wh.Run(fixtures.GithubRepo, sha)
		assert.Equal(t, whDir, repoDir)
		assert.NoError(t, err)
		whExecutor.VerifyWasCalledOnce().Execute(testHook, fixtures.GithubRepo, repoDir)
	})
	t.Run("success hooks not in cfg", func(t *testing.T) {
		preWorkflowHooksSetup(t)
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
		wh.GlobalCfg = globalCfg
		whDir, err := wh.Run(fixtures.GithubRepo, sha)
		assert.NoError(t, err)
		assert.Equal(t, whDir, "")
		whExecutor.VerifyWasCalled(Never()).Execute(testHook, fixtures.GithubRepo, repoDir)
		whWorkingDir.VerifyWasCalled(Never()).Clone(fixtures.GithubRepo, sha, repoDir)
		whWorkingDir.VerifyWasCalled(Never()).GenerateDirPath(fixtures.GithubRepo.FullName)
	})
	t.Run("error cloning", func(t *testing.T) {
		preWorkflowHooksSetup(t)
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
		wh.GlobalCfg = globalCfg
		When(whWorkingDir.GenerateDirPath(fixtures.GithubRepo.FullName)).ThenReturn(repoDir)
		When(whWorkingDir.Clone(fixtures.GithubRepo, sha, repoDir)).ThenReturn(errors.New("some error"))
		whDir, err := wh.Run(fixtures.GithubRepo, sha)
		assert.Error(t, err, "error not nil")
		assert.Empty(t, whDir)

		whExecutor.VerifyWasCalled(Never()).Execute(testHook, fixtures.GithubRepo, repoDir)
	})
	t.Run("error running pre hook", func(t *testing.T) {
		preWorkflowHooksSetup(t)
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
		wh.GlobalCfg = globalCfg
		When(whWorkingDir.GenerateDirPath(fixtures.GithubRepo.FullName)).ThenReturn(repoDir)
		When(whWorkingDir.Clone(fixtures.GithubRepo, sha, repoDir)).ThenReturn(nil)
		When(whExecutor.Execute(testHook, fixtures.GithubRepo, repoDir)).ThenReturn(errors.New("some error"))
		whDir, err := wh.Run(fixtures.GithubRepo, sha)
		assert.Error(t, err, "error not nil")
		assert.Empty(t, whDir)
	})
}
