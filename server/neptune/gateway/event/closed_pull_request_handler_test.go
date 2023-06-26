package event_test

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/legacy/http"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/models"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event"
	"github.com/runatlantis/atlantis/server/neptune/lyft/feature"
	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally/v4"
	"go.temporal.io/api/serviceerror"
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
	signaler := &testCloseSignaler{
		t:                t,
		expectedRepoName: "repo",
		expectedPullNum:  1,
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
	signaler := &testCloseSignaler{}
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
	signaler := &testCloseSignaler{}
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
	signaler := &testCloseSignaler{
		t:                t,
		err:              assert.AnError,
		expectedRepoName: "repo",
		expectedPullNum:  1,
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
	signaler := &testCloseSignaler{
		t:                t,
		expectedRepoName: "repo",
		expectedPullNum:  1,
		err:              errors.Wrap(serviceerror.NewNotFound(""), "error wrapping"),
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

type testCloseSignaler struct {
	t                *testing.T
	called           bool
	err              error
	expectedRepoName string
	expectedPullNum  int
}

func (c *testCloseSignaler) SendCloseSignal(ctx context.Context, repoName string, pullNum int) error {
	c.called = true
	assert.Equal(c.t, c.expectedRepoName, repoName)
	assert.Equal(c.t, c.expectedPullNum, pullNum)
	return c.err
}
