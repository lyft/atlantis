package command

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/legacy/core/runtime/cache"
)

type execBuilder struct {
	defaultVersion *version.Version
	versionCache   cache.ExecutionVersionCache
}

func (e *execBuilder) Build(_ context.Context, v *version.Version, path string, subCommand *SubCommand) (*exec.Cmd, error) {
	if v == nil {
		v = e.defaultVersion
	}

	binPath, err := e.versionCache.Get(v)
	if err != nil {
		return nil, errors.Wrapf(err, "getting version from cache %s", v.String())
	}

	tfCmd := fmt.Sprintf("%s %s", binPath, strings.Join(subCommand.Build(), " "))
	cmd := exec.Command("sh", "-c", tfCmd)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	cmd.Dir = path
	cmd.Env = os.Environ()
	return cmd, nil
}
