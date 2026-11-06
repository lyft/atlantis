package workflows

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/worker"
)

type nopLogger struct{}

func (nopLogger) Debug(string, ...interface{}) {}
func (nopLogger) Info(string, ...interface{})  {}
func (nopLogger) Warn(string, ...interface{})  {}
func (nopLogger) Error(string, ...interface{}) {}

// replayAll replays every recorded history in dir against the current PRRevision workflow code. A
// non-determinism error means workflows that are running during a deploy would fail to replay.
func replayAll(t *testing.T, dir string) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	assert.NoError(t, err)
	assert.NotEmpty(t, files, "no histories found in %s", dir)

	for _, f := range files {
		f := f
		t.Run(filepath.Base(f), func(t *testing.T) {
			replayer := worker.NewWorkflowReplayer()
			replayer.RegisterWorkflow(PRRevision)
			assert.NoError(t, replayer.ReplayWorkflowHistoryFromJSONFile(nopLogger{}, f))
		})
	}
}

// Histories recorded by the workflow code before ConcurrentPRRevision was introduced, against a local Temporal
// server. Workflows already running when this change deploys must keep replaying through the original behavior.
// inversion: a later PR's file listing finishes before an earlier PR's, which changes when SetPRRevision is
// scheduled. automated_first: an automated PR is listed first, which changes the task queue of later PRs.
func TestReplay_HistoriesRecordedBeforeConcurrentPRRevision(t *testing.T) {
	replayAll(t, "testdata/replay/original")
}

// Histories recorded by the current workflow code. Guards against later changes breaking replay of workflows
// started on this version.
func TestReplay_HistoriesRecordedByConcurrentPRRevision(t *testing.T) {
	replayAll(t, "testdata/replay/fixed")
}
