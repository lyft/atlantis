package preworkflow

import (
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models"
)

type HookExecutor interface {
	Execute(hook *valid.PreWorkflowHook, repo models.Repo, path string) error
}

type RepoGenerator interface {
	Clone(baseRepo models.Repo, sha string, destination string) error
	DeleteClone(filePath string) error
	GenerateDirPath(repoName string) string
}

// PreWorkflowHooksRunner is the first step when processing a workflow hook commands.
type PreWorkflowHooksRunner struct {
	GlobalCfg     valid.GlobalCfg
	HookExecutor  HookExecutor
	RepoGenerator RepoGenerator
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
	repoDir := r.RepoGenerator.GenerateDirPath(baseRepo.FullName)
	err := r.RepoGenerator.Clone(baseRepo, sha, repoDir)
	if err != nil {
		return "", errors.Wrap(err, "cloning repository")
	}

	// uses default zero values for some field in PreWorkflowHookCommandContext struct since they aren't relevant to fxn
	for _, hook := range preWorkflowHooks {
		err = r.HookExecutor.Execute(hook, baseRepo, repoDir)
		if err != nil {
			// attempt clone deletion upon failed workflow execution
			r.RepoGenerator.DeleteClone(repoDir)
			return "", errors.Wrap(err, "running pre workflow hooks")
		}
	}

	return repoDir, nil
}
