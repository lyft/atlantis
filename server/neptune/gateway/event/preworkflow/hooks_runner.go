package preworkflow

import (
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event/source"
)

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_hooks_runner.go HooksRunner
type HooksRunner interface {
	Run(repo models.Repo, sha string) (string, error)
}

// PreWorkflowHooksRunner is the first step when processing a workflow hook commands.
type PreWorkflowHooksRunner struct {
	WorkingDir   source.TmpWorkingDir
	GlobalCfg    valid.GlobalCfg
	HookExecutor HookExecutor
}

func (r *PreWorkflowHooksRunner) Run(baseRepo models.Repo, sha string) (string, error) {
	preWorkflowHooks := make([]*valid.PreWorkflowHook, 0)
	for _, repo := range r.GlobalCfg.Repos {
		if repo.IDMatches(baseRepo.ID()) && len(repo.PreWorkflowHooks) > 0 {
			preWorkflowHooks = append(preWorkflowHooks, repo.PreWorkflowHooks...)
		}
	}

	// short circuit any other calls if there are no pre-hooks configured
	if len(preWorkflowHooks) == 0 {
		return "", nil
	}
	repoDir := r.WorkingDir.GenerateDirPath(baseRepo.FullName)
	err := r.WorkingDir.Clone(baseRepo, sha, repoDir)
	if err != nil {
		return "", errors.Wrap(err, "cloning repository")
	}

	// uses default zero values for some field in PreWorkflowHookCommandContext struct since they aren't relevant to fxn
	for _, hook := range preWorkflowHooks {
		err = r.HookExecutor.Execute(hook, baseRepo, repoDir)
		if err != nil {
			return "", errors.Wrap(err, "running pre workflow hooks")
		}
	}

	return repoDir, nil
}

type MockSuccessPreWorkflowHooksRunner struct{}

func (m *MockSuccessPreWorkflowHooksRunner) Run(baseRepo models.Repo, sha string) (string, error) {
	return "", nil
}
