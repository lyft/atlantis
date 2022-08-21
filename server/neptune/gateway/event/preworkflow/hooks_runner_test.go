package preworkflow_test

import "testing"

//var wh preworkflow.PreWorkflowHookRunner
//var whWorkingDir *mocks.MockTmpWorkingDir
//
//func preWorkflowHooksSetup(t *testing.T) {
//	RegisterMockTestingT(t)
//	whWorkingDir = mocks.NewMockTmpWorkingDir()
//	whPreWorkflowHookRunner = runtime_mocks.NewMockPreWorkflowHookRunner()
//
//	wh = preworkflow.PreWorkflowHookRunner{
//		WorkingDir: whWorkingDir,
//	}
//}

func TestRunPreHooks_Clone(t *testing.T) {
	//sha := "1234"
	//ctx := context.Background()
	//testHook := valid.PreWorkflowHook{
	//	StepName:   "test",
	//	RunCommand: "some command",
	//}
	//repoDir := "path/to/repo"
	//result := "some result"
	//
	//t.Run("success hooks in cfg", func(t *testing.T) {
	//	preWorkflowHooksSetup(t)
	//	globalCfg := valid.GlobalCfg{
	//		Repos: []valid.Repo{
	//			{
	//				ID: fixtures.GithubRepo.ID(),
	//				PreWorkflowHooks: []*valid.PreWorkflowHook{
	//					&testHook,
	//				},
	//			},
	//		},
	//	}
	//
	//	wh.GlobalCfg = globalCfg
	//
	//	When(whWorkingDir.Clone(fixtures.GithubRepo, sha, repoDir)).ThenReturn(nil)
	//	//When(whPreWorkflowHookRunner.Run(ctx, pCtx, testHook.RunCommand, repoDir)).ThenReturn(result, nil)
	//
	//	err := wh.RunPreHooks(fixtures.GithubRepo, sha)
	//	assert.NoError(t, err)
	//	whPreWorkflowHookRunner.VerifyWasCalledOnce().Run(ctx, pCtx, testHook.RunCommand, repoDir)
	//})
	//t.Run("success hooks not in cfg", func(t *testing.T) {
	//	preWorkflowHooksSetup(t)
	//	globalCfg := valid.GlobalCfg{
	//		Repos: []valid.Repo{
	//			// one with hooks but mismatched id
	//			{
	//				ID: "id1",
	//				PreWorkflowHooks: []*valid.PreWorkflowHook{
	//					&testHook,
	//				},
	//			},
	//			// one with the correct id but no hooks
	//			{
	//				ID:               fixtures.GithubRepo.ID(),
	//				PreWorkflowHooks: []*valid.PreWorkflowHook{},
	//			},
	//		},
	//	}
	//
	//	wh.GlobalCfg = globalCfg
	//
	//	err := wh.RunPreHooks(fixtures.GithubRepo, sha)
	//
	//	Ok(t, err)
	//
	//	whPreWorkflowHookRunner.VerifyWasCalled(Never()).Run(ctx, pCtx, testHook.RunCommand, repoDir)
	//	whWorkingDir.VerifyWasCalled(Never()).CloneFromSha(log, fixtures.GithubRepo, sha, events.DefaultWorkspace)
	//})
	//t.Run("error locking work dir", func(t *testing.T) {
	//	preWorkflowHooksSetup(t)
	//
	//	globalCfg := valid.GlobalCfg{
	//		Repos: []valid.Repo{
	//			{
	//				ID: fixtures.GithubRepo.ID(),
	//				PreWorkflowHooks: []*valid.PreWorkflowHook{
	//					&testHook,
	//				},
	//			},
	//		},
	//	}
	//
	//	wh.GlobalCfg = globalCfg
	//
	//	err := wh.RunPreHooks(fixtures.GithubRepo, sha)
	//
	//	Assert(t, err != nil, "error not nil")
	//	whPreWorkflowHookRunner.VerifyWasCalled(Never()).Run(ctx, pCtx, testHook.RunCommand, repoDir)
	//	whWorkingDir.VerifyWasCalled(Never()).CloneFromSha(log, fixtures.GithubRepo, sha, events.DefaultWorkspace)
	//})
	//t.Run("error cloning", func(t *testing.T) {
	//	preWorkflowHooksSetup(t)
	//
	//	var unlockCalled *bool = Bool(false)
	//	unlockFn := func() {
	//		unlockCalled = Bool(true)
	//	}
	//
	//	globalCfg := valid.GlobalCfg{
	//		Repos: []valid.Repo{
	//			{
	//				ID: fixtures.GithubRepo.ID(),
	//				PreWorkflowHooks: []*valid.PreWorkflowHook{
	//					&testHook,
	//				},
	//			},
	//		},
	//	}
	//
	//	wh.GlobalCfg = globalCfg
	//
	//	When(whWorkingDir.CloneFromSha(log, fixtures.GithubRepo, sha, events.DefaultWorkspace)).ThenReturn(repoDir, errors.New("some error"))
	//
	//	err := wh.RunPreHooks(fixtures.GithubRepo, sha)
	//
	//	Assert(t, err != nil, "error not nil")
	//
	//	whPreWorkflowHookRunner.VerifyWasCalled(Never()).Run(ctx, pCtx, testHook.RunCommand, repoDir)
	//	Assert(t, *unlockCalled == true, "unlock function called")
	//})
	//t.Run("error running pre hook", func(t *testing.T) {
	//	preWorkflowHooksSetup(t)
	//
	//	var unlockCalled *bool = Bool(false)
	//	unlockFn := func() {
	//		unlockCalled = Bool(true)
	//	}
	//
	//	globalCfg := valid.GlobalCfg{
	//		Repos: []valid.Repo{
	//			{
	//				ID: fixtures.GithubRepo.ID(),
	//				PreWorkflowHooks: []*valid.PreWorkflowHook{
	//					&testHook,
	//				},
	//			},
	//		},
	//	}
	//
	//	wh.GlobalCfg = globalCfg
	//
	//	When(whWorkingDir.CloneFromSha(log, fixtures.GithubRepo, sha, events.DefaultWorkspace)).ThenReturn(repoDir, nil)
	//	When(whPreWorkflowHookRunner.Run(ctx, pCtx, testHook.RunCommand, repoDir)).ThenReturn(result, errors.New("some error"))
	//
	//	err := wh.RunPreHooks(fixtures.GithubRepo, sha)
	//
	//	Assert(t, err != nil, "error not nil")
	//	Assert(t, *unlockCalled == true, "unlock function called")
	//})
}
