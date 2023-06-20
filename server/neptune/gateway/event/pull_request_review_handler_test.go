package event_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	buffered "github.com/runatlantis/atlantis/server/legacy/http"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/models"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event"
	"github.com/runatlantis/atlantis/server/neptune/lyft/feature"
	"github.com/runatlantis/atlantis/server/neptune/sync"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally/v4"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

const (
	repoFullName = "repo"
	ref          = "ref"
)

func buildRequest(t *testing.T) *buffered.BufferedRequest {
	requestBody := "body"
	rawRequest, err := http.NewRequest(http.MethodPost, "", io.NopCloser(bytes.NewBuffer([]byte(requestBody))))
	assert.NoError(t, err)
	r, err := buffered.NewBufferedRequest(rawRequest)
	assert.NoError(t, err)
	return r
}

func TestPullRequestReviewWorkerProxy_HandleSuccessWithFailedPolicies(t *testing.T) {
	writer := &mockSnsWriter{}
	mockFetcher := &mockCheckRunFetcher{
		failedPolicies: []string{"failed policy"},
	}
	logger := logging.NewNoopCtxLogger(t)
	allocator := &testAllocator{
		t:                  t,
		expectedFeatureID:  feature.LegacyDeprecation,
		expectedFeatureCtx: feature.FeatureContext{RepoName: repoFullName},
		expectedAllocation: true,
	}
	signaler := &testSignaler{
		t:                  t,
		expectedWorkflowID: "repo||0",
		expectedRunID:      "",
		expectedSignalName: "pr-approval",
		expectedSignalArg:  workflows.PRApprovalRequest{Revision: ref},
		expectedOptions:    client.StartWorkflowOptions{},
	}
	proxy := event.PullRequestReviewWorkerProxy{
		Scheduler:          &sync.SynchronousScheduler{Logger: logger},
		SnsWriter:          writer,
		Logger:             logger,
		CheckRunFetcher:    mockFetcher,
		Allocator:          allocator,
		PRApprovalSignaler: signaler,
		Scope:              tally.NewTestScope("", map[string]string{}),
	}
	prrEvent := event.PullRequestReview{
		State: event.Approved,
		Repo:  models.Repo{FullName: repoFullName},
		Ref:   "ref",
	}
	err := proxy.Handle(context.Background(), prrEvent, buildRequest(t))
	assert.NoError(t, err)
	assert.True(t, writer.isCalled)
	assert.True(t, mockFetcher.isCalled)
	assert.True(t, signaler.called)
}

func TestPullRequestReviewWorkerProxy_HandleSuccessNoFailedPolicies(t *testing.T) {
	writer := &mockSnsWriter{}
	mockFetcher := &mockCheckRunFetcher{}
	logger := logging.NewNoopCtxLogger(t)
	allocator := &testAllocator{
		t:                  t,
		expectedFeatureID:  feature.LegacyDeprecation,
		expectedFeatureCtx: feature.FeatureContext{RepoName: repoFullName},
		expectedAllocation: true,
	}
	signaler := &testSignaler{
		t:                  t,
		expectedWorkflowID: "repo||0",
		expectedRunID:      "",
		expectedErr:        serviceerror.NewNotFound("workflow not found"),
		expectedSignalName: "pr-approval",
		expectedSignalArg:  workflows.PRApprovalRequest{Revision: ref},
		expectedOptions:    client.StartWorkflowOptions{},
	}
	proxy := event.PullRequestReviewWorkerProxy{
		Scheduler:          &sync.SynchronousScheduler{Logger: logger},
		SnsWriter:          writer,
		Logger:             logger,
		CheckRunFetcher:    mockFetcher,
		Allocator:          allocator,
		PRApprovalSignaler: signaler,
		Scope:              tally.NewTestScope("", map[string]string{}),
	}
	prrEvent := event.PullRequestReview{
		State: event.Approved,
		Repo:  models.Repo{FullName: repoFullName},
		Ref:   ref,
	}
	err := proxy.Handle(context.Background(), prrEvent, buildRequest(t))
	assert.NoError(t, err)
	assert.False(t, writer.isCalled)
	assert.True(t, mockFetcher.isCalled)
	assert.True(t, signaler.called)
}

func TestPullRequestReviewWorkerProxy_NotApprovalEvent(t *testing.T) {
	writer := &mockSnsWriter{}
	mockFetcher := &mockCheckRunFetcher{}
	logger := logging.NewNoopCtxLogger(t)
	signaler := &testSignaler{}
	proxy := event.PullRequestReviewWorkerProxy{
		Scheduler:          &sync.SynchronousScheduler{Logger: logger},
		SnsWriter:          writer,
		Logger:             logger,
		CheckRunFetcher:    mockFetcher,
		PRApprovalSignaler: signaler,
		Scope:              tally.NewTestScope("", map[string]string{}),
	}
	prrEvent := event.PullRequestReview{
		State: "something else",
		Repo:  models.Repo{FullName: repoFullName},
	}
	err := proxy.Handle(context.Background(), prrEvent, buildRequest(t))
	assert.NoError(t, err)
	assert.False(t, writer.isCalled)
	assert.False(t, mockFetcher.isCalled)
	assert.False(t, signaler.called)
}

func TestPullRequestReviewWorkerProxy_FetcherError(t *testing.T) {
	writer := &mockSnsWriter{}
	mockFetcher := &mockCheckRunFetcher{
		err: assert.AnError,
	}
	logger := logging.NewNoopCtxLogger(t)
	allocator := &testAllocator{
		t:                  t,
		expectedFeatureID:  feature.LegacyDeprecation,
		expectedFeatureCtx: feature.FeatureContext{RepoName: repoFullName},
		expectedAllocation: true,
	}
	signaler := &testSignaler{
		t:                  t,
		expectedWorkflowID: "repo||0",
		expectedRunID:      "",
		expectedSignalName: "pr-approval",
		expectedSignalArg:  workflows.PRApprovalRequest{Revision: ref},
		expectedOptions:    client.StartWorkflowOptions{},
	}
	proxy := event.PullRequestReviewWorkerProxy{
		Scheduler:          &sync.SynchronousScheduler{Logger: logger},
		SnsWriter:          writer,
		Logger:             logger,
		CheckRunFetcher:    mockFetcher,
		Allocator:          allocator,
		PRApprovalSignaler: signaler,
		Scope:              tally.NewTestScope("", map[string]string{}),
	}
	prrEvent := event.PullRequestReview{
		State: event.Approved,
		Repo:  models.Repo{FullName: repoFullName},
		Ref:   ref,
	}
	err := proxy.Handle(context.Background(), prrEvent, buildRequest(t))
	assert.Error(t, err)
	assert.False(t, writer.isCalled)
	assert.True(t, mockFetcher.isCalled)
	assert.True(t, signaler.called)
}

func TestPullRequestReviewWorkerProxy_SNSError(t *testing.T) {
	writer := &mockSnsWriter{}
	mockFetcher := &mockCheckRunFetcher{
		failedPolicies: []string{"failed policy"},
	}
	logger := logging.NewNoopCtxLogger(t)
	allocator := &testAllocator{
		t:                  t,
		expectedFeatureID:  feature.LegacyDeprecation,
		expectedFeatureCtx: feature.FeatureContext{RepoName: repoFullName},
		expectedAllocation: true,
	}
	signaler := &testSignaler{
		t:                  t,
		expectedWorkflowID: "repo||0",
		expectedRunID:      "",
		expectedSignalName: "pr-approval",
		expectedSignalArg:  workflows.PRApprovalRequest{Revision: ref},
		expectedOptions:    client.StartWorkflowOptions{},
	}
	proxy := event.PullRequestReviewWorkerProxy{
		Scheduler:          &sync.SynchronousScheduler{Logger: logger},
		SnsWriter:          writer,
		Logger:             logger,
		CheckRunFetcher:    mockFetcher,
		Allocator:          allocator,
		PRApprovalSignaler: signaler,
		Scope:              tally.NewTestScope("", map[string]string{}),
	}
	prrEvent := event.PullRequestReview{
		State: event.Approved,
		Repo:  models.Repo{FullName: repoFullName},
		Ref:   ref,
	}

	err := proxy.Handle(context.Background(), prrEvent, buildRequest(t))
	assert.NoError(t, err)
	assert.True(t, writer.isCalled)
	assert.True(t, mockFetcher.isCalled)
	assert.True(t, signaler.called)
}

func TestPullRequestReviewWorkerProxy_SignalerError(t *testing.T) {
	writer := &mockSnsWriter{}
	mockFetcher := &mockCheckRunFetcher{
		failedPolicies: []string{"failed policy"},
	}
	logger := logging.NewNoopCtxLogger(t)
	allocator := &testAllocator{
		t:                  t,
		expectedFeatureID:  feature.LegacyDeprecation,
		expectedFeatureCtx: feature.FeatureContext{RepoName: repoFullName},
		expectedAllocation: true,
	}
	signaler := &testSignaler{
		t:                  t,
		expectedWorkflowID: "repo||0",
		expectedRunID:      "",
		expectedSignalName: "pr-approval",
		expectedSignalArg:  workflows.PRApprovalRequest{Revision: ref},
		expectedErr:        assert.AnError,
		expectedOptions:    client.StartWorkflowOptions{},
	}
	proxy := event.PullRequestReviewWorkerProxy{
		Scheduler:          &sync.SynchronousScheduler{Logger: logger},
		SnsWriter:          writer,
		Logger:             logger,
		CheckRunFetcher:    mockFetcher,
		Allocator:          allocator,
		PRApprovalSignaler: signaler,
		Scope:              tally.NewTestScope("", map[string]string{}),
	}
	prrEvent := event.PullRequestReview{
		State: event.Approved,
		Repo:  models.Repo{FullName: repoFullName},
		Ref:   ref,
	}

	err := proxy.Handle(context.Background(), prrEvent, buildRequest(t))
	assert.Error(t, err)
	assert.True(t, writer.isCalled)
	assert.True(t, mockFetcher.isCalled)
	assert.True(t, signaler.called)
}

type mockSnsWriter struct {
	err      error
	isCalled bool
}

func (s *mockSnsWriter) WriteWithContext(ctx context.Context, payload []byte) error {
	s.isCalled = true
	return s.err
}

type mockCheckRunFetcher struct {
	failedPolicies []string
	err            error
	isCalled       bool
}

func (f *mockCheckRunFetcher) ListFailedPolicyCheckRunNames(_ context.Context, _ int64, _ models.Repo, _ string) ([]string, error) {
	f.isCalled = true
	return f.failedPolicies, f.err
}
