package event_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/legacy/http"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/models"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event"
	"github.com/stretchr/testify/assert"
)

func TestLegacyHandler_Handle_NoRoots(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	statusUpdater := &mockVCSStatusUpdater{}
	workerProxy := &mockWorkerProxy{}
	legacyHandler := event.LegacyPullHandler{
		Logger:           logger,
		VCSStatusUpdater: statusUpdater,
		WorkerProxy:      workerProxy,
	}
	err := legacyHandler.Handle(context.Background(), &http.BufferedRequest{}, event.PullRequest{}, []*valid.MergedProjectCfg{})
	assert.NoError(t, err)
	assert.False(t, workerProxy.called)
	assert.Equal(t, statusUpdater.combinedCountCalls, 1)
	assert.Equal(t, statusUpdater.combinedCalls, 0)
}

func TestLegacyHandler_Handle_WorkerProxyFailure(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	statusUpdater := &mockVCSStatusUpdater{}
	legacyRoot := &valid.MergedProjectCfg{
		Name: "legacy",
	}
	legacyHandler := event.LegacyPullHandler{
		Logger:           logger,
		VCSStatusUpdater: statusUpdater,
		WorkerProxy:      &mockWorkerProxy{err: assert.AnError},
	}
	err := legacyHandler.Handle(context.Background(), &http.BufferedRequest{}, event.PullRequest{}, []*valid.MergedProjectCfg{legacyRoot})
	assert.ErrorIs(t, err, assert.AnError)
	assert.Equal(t, 0, statusUpdater.combinedCountCalls)
	assert.Equal(t, 0, statusUpdater.combinedCalls)
}

func TestLegacyHandler_Handle_WorkerProxySuccess(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	statusUpdater := &mockVCSStatusUpdater{}
	workerProxy := &mockWorkerProxy{}
	legacyRoot := &valid.MergedProjectCfg{
		Name: "legacy",
	}
	legacyHandler := event.LegacyPullHandler{
		Logger:           logger,
		VCSStatusUpdater: statusUpdater,
		WorkerProxy:      workerProxy,
	}
	err := legacyHandler.Handle(context.Background(), &http.BufferedRequest{}, event.PullRequest{}, []*valid.MergedProjectCfg{legacyRoot})
	assert.NoError(t, err)
	assert.True(t, workerProxy.called)
	assert.Equal(t, 0, statusUpdater.combinedCountCalls)
	assert.Equal(t, 0, statusUpdater.combinedCalls)
}

type mockVCSStatusUpdater struct {
	combinedCalls int
	combinedError error

	combinedCountError error
	combinedCountCalls int
}

func (m *mockVCSStatusUpdater) UpdateCombined(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.VCSStatus, cmdName fmt.Stringer, statusID string, output string) (string, error) {
	m.combinedCalls++
	return "", m.combinedError
}

func (m *mockVCSStatusUpdater) UpdateCombinedCount(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.VCSStatus, cmdName fmt.Stringer, numSuccess int, numTotal int, statusID string) (string, error) {
	m.combinedCountCalls++
	return "", m.combinedCountError
}

type mockWorkerProxy struct {
	called bool
	err    error
}

func (w *mockWorkerProxy) Handle(ctx context.Context, request *http.BufferedRequest, event event.PullRequest) error {
	w.called = true
	return w.err
}
