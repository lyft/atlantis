package runtime

import (
	version "github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"
)


type VersionedExecutorWorkflow interface {
	ExecutorVersionEnsurer
	ExecutorArgsResolver
	Executor
}

// Executor runs an executable with provided environment variables and arguments and returns stdout
type Executor interface {
	Run(log *logging.SimpleLogger, executablePath string, envs map[string]string, args []string) (string, error)
}

// ExecutorVersionEnsurer ensures a given version exists and outputs a path to the executable
type ExecutorVersionEnsurer interface {
	EnsureExecutorVersion(log *logging.SimpleLogger, v *version.Version) (string, error)
}

// ExecutorArgsBuilder builds an arg string
type ExecutorArgsResolver interface {
	ResolveArgs(ctx models.ProjectCommandContext) ([]string, error)
}
