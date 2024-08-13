package event_test

import (
	"bytes"
	"context"
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/neptune/gateway/config"
	"github.com/runatlantis/atlantis/server/neptune/gateway/pr"
	"io"
	"net/http"
	"testing"

	buffered "github.com/runatlantis/atlantis/server/legacy/http"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/models"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event"
	"github.com/runatlantis/atlantis/server/neptune/sync"
	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally/v4"
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

func TestPullRequestReviewWorkerProxy_HandleApprovalWithFailedPolicies(t *testing.T) {
	writer := &mockSnsWriter{}
	mockFetcher := &mockCheckRunFetcher{
		failedPolicies: []string{"failed policy"},
	}
	logger := logging.NewNoopCtxLogger(t)
	signaler := &reviewSignaler{
		t:                t,
		expectedRepoName: "repo",
		expectedPullNum:  0,
		expectedRevision: ref,
	}
	proxy := event.PullRequestReviewWorkerProxy{
		Scheduler:        &sync.SynchronousScheduler{Logger: logger},
		SnsWriter:        writer,
		Logger:           logger,
		CheckRunFetcher:  mockFetcher,
		WorkflowSignaler: signaler,
		Scope:            tally.NewTestScope("", map[string]string{}),
	}
	prrEvent := event.PullRequestReview{
		State: event.Approved,
		Repo:  models.Repo{FullName: repoFullName},
		Ref:   "ref",
	}
	err := proxy.Handle(context.Background(), prrEvent, buildRequest(t))
	assert.NoError(t, err)
	assert.True(t, writer.isCalled)
	assert.True(t, mockFetcher.called)
	assert.True(t, signaler.called)
}

func TestPullRequestReviewWorkerProxy_HandleChangesRequestedWithFailedPolicies(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	signaler := &reviewSignaler{
		t:                t,
		expectedRepoName: "repo",
		expectedPullNum:  1,
		expectedRevision: "1234",
	}
	mockFetcher := &mockCheckRunFetcher{
		failedPolicies: []string{"failed policy"},
	}
	expectedCommit := &config.RepoCommit{
		Repo:          testRepo,
		Branch:        "somebranch",
		Sha:           "1234",
		OptionalPRNum: 1,
	}
	rootConfigBuilder := &mockConfigBuilder{
		expectedCommit: expectedCommit,
		expectedT:      t,
		rootConfigs:    []*valid.MergedProjectCfg{},
	}
	proxy := event.PullRequestReviewWorkerProxy{
		Scheduler:         &sync.SynchronousScheduler{Logger: logger},
		Logger:            logger,
		WorkflowSignaler:  signaler,
		Scope:             tally.NewTestScope("", map[string]string{}),
		RootConfigBuilder: rootConfigBuilder,
		GlobalCfg:         valid.GlobalCfg{},
		CheckRunFetcher:   mockFetcher,
	}
	pull := models.PullRequest{
		HeadRepo:   testRepo,
		HeadBranch: "somebranch",
		HeadCommit: "1234",
		Num:        1,
	}
	prrEvent := event.PullRequestReview{
		State: event.ChangesRequested,
		Repo:  models.Repo{FullName: repoFullName},
		Ref:   "1234",
		Pull:  pull,
	}
	err := proxy.Handle(context.Background(), prrEvent, buildRequest(t))
	assert.NoError(t, err)
	assert.True(t, mockFetcher.called)
	assert.True(t, signaler.called)
}

func TestPullRequestReviewWorkerProxy_HandleSuccessNoFailedPolicies(t *testing.T) {
	writer := &mockSnsWriter{}
	mockFetcher := &mockCheckRunFetcher{}
	logger := logging.NewNoopCtxLogger(t)
	signaler := &reviewSignaler{
		t:                t,
		expectedRepoName: "repo",
		expectedPullNum:  0,
		expectedRevision: ref,
	}
	proxy := event.PullRequestReviewWorkerProxy{
		Scheduler:        &sync.SynchronousScheduler{Logger: logger},
		SnsWriter:        writer,
		Logger:           logger,
		CheckRunFetcher:  mockFetcher,
		WorkflowSignaler: signaler,
		Scope:            tally.NewTestScope("", map[string]string{}),
	}
	prrEvent := event.PullRequestReview{
		State: event.Approved,
		Repo:  models.Repo{FullName: repoFullName},
		Ref:   ref,
	}
	err := proxy.Handle(context.Background(), prrEvent, buildRequest(t))
	assert.NoError(t, err)
	assert.False(t, writer.isCalled)
	assert.True(t, mockFetcher.called)
	assert.True(t, signaler.called)
}

func TestPullRequestReviewWorkerProxy_InvalidEvent(t *testing.T) {
	writer := &mockSnsWriter{}
	mockFetcher := &mockCheckRunFetcher{}
	logger := logging.NewNoopCtxLogger(t)
	signaler := &reviewSignaler{}
	proxy := event.PullRequestReviewWorkerProxy{
		Scheduler:        &sync.SynchronousScheduler{Logger: logger},
		SnsWriter:        writer,
		Logger:           logger,
		CheckRunFetcher:  mockFetcher,
		WorkflowSignaler: signaler,
		Scope:            tally.NewTestScope("", map[string]string{}),
	}
	prrEvent := event.PullRequestReview{
		State: "something else",
		Repo:  models.Repo{FullName: repoFullName},
	}
	err := proxy.Handle(context.Background(), prrEvent, buildRequest(t))
	assert.NoError(t, err)
	assert.False(t, writer.isCalled)
	assert.False(t, mockFetcher.called)
	assert.False(t, signaler.called)
}

func TestPullRequestReviewWorkerProxy_FetcherError(t *testing.T) {
	writer := &mockSnsWriter{}
	mockFetcher := &mockCheckRunFetcher{
		err: assert.AnError,
	}
	logger := logging.NewNoopCtxLogger(t)
	signaler := &reviewSignaler{
		t:                t,
		expectedRepoName: "repo",
		expectedPullNum:  0,
		expectedRevision: ref,
	}
	proxy := event.PullRequestReviewWorkerProxy{
		Scheduler:        &sync.SynchronousScheduler{Logger: logger},
		SnsWriter:        writer,
		Logger:           logger,
		CheckRunFetcher:  mockFetcher,
		WorkflowSignaler: signaler,
		Scope:            tally.NewTestScope("", map[string]string{}),
	}
	prrEvent := event.PullRequestReview{
		State: event.Approved,
		Repo:  models.Repo{FullName: repoFullName},
		Ref:   ref,
	}
	err := proxy.Handle(context.Background(), prrEvent, buildRequest(t))
	assert.Error(t, err)
	assert.False(t, writer.isCalled)
	assert.True(t, mockFetcher.called)
	assert.True(t, signaler.called)
}

func TestPullRequestReviewWorkerProxy_SNSError(t *testing.T) {
	writer := &mockSnsWriter{}
	mockFetcher := &mockCheckRunFetcher{
		failedPolicies: []string{"failed policy"},
	}
	logger := logging.NewNoopCtxLogger(t)
	signaler := &reviewSignaler{
		t:                t,
		expectedRepoName: "repo",
		expectedPullNum:  0,
		expectedRevision: ref,
	}
	proxy := event.PullRequestReviewWorkerProxy{
		Scheduler:        &sync.SynchronousScheduler{Logger: logger},
		SnsWriter:        writer,
		Logger:           logger,
		CheckRunFetcher:  mockFetcher,
		WorkflowSignaler: signaler,
		Scope:            tally.NewTestScope("", map[string]string{}),
	}
	prrEvent := event.PullRequestReview{
		State: event.Approved,
		Repo:  models.Repo{FullName: repoFullName},
		Ref:   ref,
	}

	err := proxy.Handle(context.Background(), prrEvent, buildRequest(t))
	assert.NoError(t, err)
	assert.True(t, writer.isCalled)
	assert.True(t, mockFetcher.called)
	assert.True(t, signaler.called)
}

func TestPullRequestReviewWorkerProxy_SignalerError(t *testing.T) {
	writer := &mockSnsWriter{}
	mockFetcher := &mockCheckRunFetcher{
		failedPolicies: []string{"failed policy"},
	}
	logger := logging.NewNoopCtxLogger(t)
	signaler := &reviewSignaler{
		t:                t,
		expectedRepoName: "repo",
		expectedPullNum:  0,
		expectedRevision: ref,
		err:              assert.AnError,
	}
	proxy := event.PullRequestReviewWorkerProxy{
		Scheduler:        &sync.SynchronousScheduler{Logger: logger},
		SnsWriter:        writer,
		Logger:           logger,
		CheckRunFetcher:  mockFetcher,
		WorkflowSignaler: signaler,
		Scope:            tally.NewTestScope("", map[string]string{}),
	}
	prrEvent := event.PullRequestReview{
		State: event.Approved,
		Repo:  models.Repo{FullName: repoFullName},
		Ref:   ref,
	}

	err := proxy.Handle(context.Background(), prrEvent, buildRequest(t))
	assert.Error(t, err)
	assert.True(t, writer.isCalled)
	assert.True(t, mockFetcher.called)
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
	called         bool
}

func (f *mockCheckRunFetcher) ListFailedPlanCheckRunNames(ctx context.Context, installationToken int64, repo models.Repo, ref string) ([]string, error) {
	f.called = true
	return f.failedPolicies, f.err
}

func (f *mockCheckRunFetcher) ListFailedPolicyCheckRunNames(_ context.Context, _ int64, _ models.Repo, _ string) ([]string, error) {
	f.called = true
	return f.failedPolicies, f.err
}

type reviewSignaler struct {
	t                *testing.T
	expectedRepoName string
	expectedPullNum  int
	expectedRevision string
	called           bool
	err              error
}

func (r *reviewSignaler) SendReviewSignal(ctx context.Context, repoName string, pullNum int, revision string) error {
	r.called = true
	assert.Equal(r.t, r.expectedRevision, revision)
	assert.Equal(r.t, r.expectedRepoName, repoName)
	assert.Equal(r.t, r.expectedPullNum, pullNum)
	return r.err
}

func (r *reviewSignaler) SendRevisionSignal(ctx context.Context, rootCfgs []*valid.MergedProjectCfg, request pr.Request) error {
	r.called = true
	assert.Equal(r.t, r.expectedRevision, request.Revision)
	assert.Equal(r.t, r.expectedRepoName, request.Repo.FullName)
	assert.Equal(r.t, r.expectedPullNum, request.Number)
	return r.err
}
