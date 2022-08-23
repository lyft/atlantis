package preworkflow

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models"
	"os"
	"os/exec"
)

// interface for actual execution makes local testing easier

type HookExecutor interface {
	Execute(hook *valid.PreWorkflowHook, repo models.Repo, path string) error
}

type PreWorkflowHookExecutor struct {
}

func (e *PreWorkflowHookExecutor) Execute(hook *valid.PreWorkflowHook, repo models.Repo, path string) error {
	cmd := exec.Command("sh", "-c", hook.RunCommand) // #nosec
	cmd.Dir = path

	baseEnvVars := os.Environ()
	customEnvVars := map[string]string{
		"BASE_BRANCH_NAME": repo.DefaultBranch,
		"BASE_REPO_NAME":   repo.FullName,
		"BASE_REPO_OWNER":  repo.Owner,
		"DIR":              path,
	}

	finalEnvVars := baseEnvVars
	for key, val := range customEnvVars {
		finalEnvVars = append(finalEnvVars, fmt.Sprintf("%s=%s", key, val))
	}
	cmd.Env = finalEnvVars

	// pre-workflow hooks operate different than our terraform steps
	// it's up to the underlying implementation to log errors/output accordingly.
	// The only required step is to share Stdout and Stderr with the underlying
	// process, so that our logging sidecar can forward the logs to kibana
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "running hook")
	}
	return nil
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
