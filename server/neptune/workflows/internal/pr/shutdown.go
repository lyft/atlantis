package pr

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision"
	"go.temporal.io/sdk/workflow"
)

type ShutdownStateChecker struct {
}

func (s ShutdownStateChecker) ShouldShutdown(ctx workflow.Context, prRevision revision.Revision) bool {
	//TODO implement me
	return false
}
