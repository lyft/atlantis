package event_test

import (
	"context"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/http"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/lyft/feature"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally/v4"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"testing"
)

func TestClosedPullHandler_Handle(t *testing.T) {
	allocator := &testAllocator{
		expectedAllocation: true,
		expectedFeatureID:  feature.LegacyDeprecation,
		expectedFeatureCtx: feature.FeatureContext{
			RepoName: repoFullName,
		},
		t: t,
	}
	workerProxy := &mockWorkerProxy{}
	signaler := &testSignaler{
		t:                  t,
		expectedWorkflowID: "repo||1",
		expectedRunID:      "",
		expectedSignalName: "pr-close",
		expectedSignalArg:  workflows.PRShutdownRequest{},
		expectedOptions:    client.StartWorkflowOptions{},
	}
	pullHandler := event.ClosedPullRequestHandler{
		Allocator:       allocator,
		Logger:          logging.NewNoopCtxLogger(t),
		WorkerProxy:     workerProxy,
		PRCloseSignaler: signaler,
	}
	pr := event.PullRequest{
		Pull: models.PullRequest{
			BaseRepo:   testRepo,
			HeadRepo:   testRepo,
			HeadBranch: "somebranch",
			HeadCommit: "1234",
			Num:        1,
		},
	}
	err := pullHandler.Handle(context.Background(), &http.BufferedRequest{}, pr)
	assert.True(t, signaler.called)
	assert.True(t, workerProxy.called)
	assert.NoError(t, err)
}

func TestClosedPullHandler_Handle_AllocationError(t *testing.T) {
	allocator := &testAllocator{
		expectedError:     assert.AnError,
		expectedFeatureID: feature.LegacyDeprecation,
		expectedFeatureCtx: feature.FeatureContext{
			RepoName: repoFullName,
		},
		t: t,
	}
	workerProxy := &mockWorkerProxy{}
	signaler := &testSignaler{}
	pullHandler := event.ClosedPullRequestHandler{
		Allocator:       allocator,
		Logger:          logging.NewNoopCtxLogger(t),
		WorkerProxy:     workerProxy,
		PRCloseSignaler: signaler,
	}
	pr := event.PullRequest{
		Pull: models.PullRequest{
			BaseRepo:   testRepo,
			HeadRepo:   testRepo,
			HeadBranch: "somebranch",
			HeadCommit: "1234",
			Num:        1,
		},
	}
	err := pullHandler.Handle(context.Background(), &http.BufferedRequest{}, pr)
	assert.False(t, signaler.called)
	assert.True(t, workerProxy.called)
	assert.NoError(t, err)
}

func TestClosedPullHandler_Handle_AllocationFail(t *testing.T) {
	allocator := &testAllocator{
		expectedFeatureID: feature.LegacyDeprecation,
		expectedFeatureCtx: feature.FeatureContext{
			RepoName: repoFullName,
		},
		t: t,
	}
	workerProxy := &mockWorkerProxy{}
	signaler := &testSignaler{}
	pullHandler := event.ClosedPullRequestHandler{
		Allocator:       allocator,
		Logger:          logging.NewNoopCtxLogger(t),
		WorkerProxy:     workerProxy,
		PRCloseSignaler: signaler,
	}
	pr := event.PullRequest{
		Pull: models.PullRequest{
			BaseRepo:   testRepo,
			HeadRepo:   testRepo,
			HeadBranch: "somebranch",
			HeadCommit: "1234",
			Num:        1,
		},
	}
	err := pullHandler.Handle(context.Background(), &http.BufferedRequest{}, pr)
	assert.False(t, signaler.called)
	assert.True(t, workerProxy.called)
	assert.NoError(t, err)
}

func TestClosedPullHandler_Handle_SignalError(t *testing.T) {
	allocator := &testAllocator{
		expectedAllocation: true,
		expectedFeatureID:  feature.LegacyDeprecation,
		expectedFeatureCtx: feature.FeatureContext{
			RepoName: repoFullName,
		},
		t: t,
	}
	workerProxy := &mockWorkerProxy{}
	signaler := &testSignaler{
		t:                  t,
		expectedWorkflowID: "repo||1",
		expectedRunID:      "",
		expectedSignalName: "pr-close",
		expectedSignalArg:  workflows.PRShutdownRequest{},
		expectedOptions:    client.StartWorkflowOptions{},
		expectedErr:        assert.AnError,
	}
	pullHandler := event.ClosedPullRequestHandler{
		Allocator:       allocator,
		Logger:          logging.NewNoopCtxLogger(t),
		WorkerProxy:     workerProxy,
		PRCloseSignaler: signaler,
	}
	pr := event.PullRequest{
		Pull: models.PullRequest{
			BaseRepo:   testRepo,
			HeadRepo:   testRepo,
			HeadBranch: "somebranch",
			HeadCommit: "1234",
			Num:        1,
		},
	}
	err := pullHandler.Handle(context.Background(), &http.BufferedRequest{}, pr)
	assert.True(t, signaler.called)
	assert.True(t, workerProxy.called)
	assert.Error(t, err)
}

func TestClosedPullHandler_Handle_SignalNotFoundError(t *testing.T) {
	allocator := &testAllocator{
		expectedAllocation: true,
		expectedFeatureID:  feature.LegacyDeprecation,
		expectedFeatureCtx: feature.FeatureContext{
			RepoName: repoFullName,
		},
		t: t,
	}
	workerProxy := &mockWorkerProxy{}
	signaler := &testSignaler{
		t:                  t,
		expectedWorkflowID: "repo||1",
		expectedRunID:      "",
		expectedSignalName: "pr-close",
		expectedSignalArg:  workflows.PRShutdownRequest{},
		expectedOptions:    client.StartWorkflowOptions{},
		expectedErr:        errors.Wrap(serviceerror.NewNotFound(""), "error wrapping"),
	}
	pullHandler := event.ClosedPullRequestHandler{
		Allocator:       allocator,
		Logger:          logging.NewNoopCtxLogger(t),
		WorkerProxy:     workerProxy,
		PRCloseSignaler: signaler,
		Scope:           tally.NewTestScope("", map[string]string{}),
	}
	pr := event.PullRequest{
		Pull: models.PullRequest{
			BaseRepo:   testRepo,
			HeadRepo:   testRepo,
			HeadBranch: "somebranch",
			HeadCommit: "1234",
			Num:        1,
		},
	}
	err := pullHandler.Handle(context.Background(), &http.BufferedRequest{}, pr)
	assert.True(t, signaler.called)
	assert.True(t, workerProxy.called)
	assert.NoError(t, err)
}
