package exec_test

import (
	"context"
	"github.com/runatlantis/atlantis/server/logging"
	subprocess_exec "github.com/runatlantis/atlantis/server/neptune/exec"
	"github.com/stretchr/testify/assert"
	"os/exec"
	"testing"
)

func TestCmd_RunWithNewProcessGroup(t *testing.T) {
	cmd := exec.Command("sleep", "1")
	subprocessCmd := subprocess_exec.Cmd{
		Cmd:    cmd,
		Logger: logging.NewNoopCtxLogger(t),
	}
	err := subprocessCmd.RunWithNewProcessGroup(context.Background())
	assert.NoError(t, err)
}

func TestCmd_RunWithNewProcessGroup_CanceledCtx(t *testing.T) {
	cmd := exec.Command("sleep", "10")
	subprocessCmd := subprocess_exec.Cmd{
		Cmd:    cmd,
		Logger: logging.NewNoopCtxLogger(t),
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := subprocessCmd.RunWithNewProcessGroup(ctx)
	assert.ErrorContains(t, err, "context canceled")
}
