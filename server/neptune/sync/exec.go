package sync

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	key "github.com/runatlantis/atlantis/server/neptune/context"
	"github.com/runatlantis/atlantis/server/neptune/logger"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// RunNewProcessGroupCommand is useful for running separate commands that shouldn't receive termination
// signals at the same time as the parent process
func RunNewProcessGroupCommand(ctx context.Context, cmd *exec.Cmd, cmdName string) error {
	// should this be defined by struct creating the cmd or this helper fxn?
	// could also replace this with a check confirming this is set and erroring if not
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, fmt.Sprintf("starting %s command", cmdName))
	}
	done := make(chan struct{})
	defer close(done)
	go TerminateProcessOnCtxCancellation(ctx, cmd.Process, done)

	err := cmd.Wait()
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), fmt.Sprintf("waiting for %s process", cmdName))
	}
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("waiting for %s process", cmdName))
	}
	return nil
}

func TerminateProcessOnCtxCancellation(ctx context.Context, p *os.Process, processDone chan struct{}) {
	select {
	case <-ctx.Done():
		// received context cancellation, terminate active process
		terminateProcess(ctx, p, processDone)
	case <-processDone:
		// process completed on its own, simply exit
	}
}

func terminateProcess(ctx context.Context, p *os.Process, processDone chan struct{}) {
	logger.Warn(ctx, "Terminating active process gracefully")
	err := p.Signal(syscall.SIGTERM)
	if err != nil {
		logger.Error(ctx, "Unable to terminate process", key.ErrKey, err)
	}

	// if we still haven't shutdown after 60 seconds, we should just kill the process
	// this ensures that we at least can gracefully shutdown other parts of the system
	// before we are killed entirely.
	kill := time.After(60 * time.Second)
	select {
	case <-kill:
		logger.Warn(ctx, "Killing terraform process since graceful shutdown is taking suspiciously long. State corruption may have occurred.")
		err := p.Signal(syscall.SIGKILL)
		if err != nil {
			logger.Error(ctx, "Unable to kill process", key.ErrKey, err)
		}
	case <-processDone:
	}
}
