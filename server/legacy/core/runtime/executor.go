package runtime

import (
	"context"

	version "github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/legacy/events/command"
	"github.com/runatlantis/atlantis/server/logging"
)

// Executor runs an executable with provided environment variables and arguments and returns stdout
type Executor interface {
	Run(ctx context.Context, prjCtx command.ProjectContext, executablePath string, envs map[string]string, workdir string, extraArgs []string) (string, error)
}

// ExecutorVersionEnsurer ensures a given version exists and outputs a path to the executable
type ExecutorVersionEnsurer interface {
	EnsureExecutorVersion(log logging.Logger, v *version.Version) (string, error)
}
