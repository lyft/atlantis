package event_test

import (
	"context"
	"fmt"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/http"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/neptune/gateway/config"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event"
	"github.com/runatlantis/atlantis/server/neptune/sync"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestModifiedPullHandler_Handle_CriteriaFailure(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	pullHandler := event.ModifiedPullHandler{
		Logger:             logger,
		Scheduler:          &sync.SynchronousScheduler{Logger: logger},
		GlobalCfg:          valid.GlobalCfg{},
		RequirementChecker: &requirementsChecker{err: assert.AnError},
	}
	err := pullHandler.Handle(context.Background(), &http.BufferedRequest{}, event.PullRequest{})
	assert.ErrorIs(t, err, assert.AnError)
}

func TestModifiedPullHandler_Handle_RootBuilderFailure(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	pullHandler := event.ModifiedPullHandler{
		Logger:             logger,
		Scheduler:          &sync.SynchronousScheduler{Logger: logger},
		GlobalCfg:          valid.GlobalCfg{},
		RequirementChecker: &requirementsChecker{},
		RootConfigBuilder: &mockConfigBuilder{
			expectedCommit: &config.RepoCommit{},
			expectedT:      t,
			error:          assert.AnError,
		},
	}
	err := pullHandler.Handle(context.Background(), &http.BufferedRequest{}, event.PullRequest{})
	assert.ErrorIs(t, err, assert.AnError)
}

func TestModifiedPullHandler_Handle_NoRoots(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	statusUpdater := &mockVCSStatusUpdater{
		combinedCountCalls: 1,
	}
	pullHandler := event.ModifiedPullHandler{
		Logger:             logger,
		Scheduler:          &sync.SynchronousScheduler{Logger: logger},
		GlobalCfg:          valid.GlobalCfg{},
		RequirementChecker: &requirementsChecker{},
		VCSStatusUpdater:   statusUpdater,
		RootConfigBuilder: &mockConfigBuilder{
			expectedCommit: &config.RepoCommit{},
			expectedT:      t,
		},
	}
	err := pullHandler.Handle(context.Background(), &http.BufferedRequest{}, event.PullRequest{})
	assert.NoError(t, err)
}

func TestModifiedPullHandler_Handle_WorkerProxyFailure(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	statusUpdater := &mockVCSStatusUpdater{
		combinedCountCalls: 1,
	}
	legacyRoot := &valid.MergedProjectCfg{
		Name:         "legacy",
		WorkflowMode: valid.DefaultWorkflowMode,
	}
	pullHandler := event.ModifiedPullHandler{
		Logger:             logger,
		Scheduler:          &sync.SynchronousScheduler{Logger: logger},
		GlobalCfg:          valid.GlobalCfg{},
		RequirementChecker: &requirementsChecker{},
		VCSStatusUpdater:   statusUpdater,
		WorkerProxy:        &mockWorkerProxy{err: assert.AnError},
		RootConfigBuilder: &mockConfigBuilder{
			expectedCommit: &config.RepoCommit{},
			expectedT:      t,
			rootConfigs:    []*valid.MergedProjectCfg{legacyRoot},
		},
	}
	err := pullHandler.Handle(context.Background(), &http.BufferedRequest{}, event.PullRequest{})
	assert.ErrorIs(t, err, assert.AnError)
}

func TestModifiedPullHandler_Handle_LegacyRoots(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	statusUpdater := &mockVCSStatusUpdater{
		combinedCountCalls: 1,
	}
	legacyRoot := &valid.MergedProjectCfg{
		Name:         "legacy",
		WorkflowMode: valid.DefaultWorkflowMode,
	}
	workerProxy := &mockWorkerProxy{}
	pullHandler := event.ModifiedPullHandler{
		Logger:             logger,
		Scheduler:          &sync.SynchronousScheduler{Logger: logger},
		GlobalCfg:          valid.GlobalCfg{},
		RequirementChecker: &requirementsChecker{},
		VCSStatusUpdater:   statusUpdater,
		WorkerProxy:        workerProxy,
		RootConfigBuilder: &mockConfigBuilder{
			expectedCommit: &config.RepoCommit{},
			expectedT:      t,
			rootConfigs:    []*valid.MergedProjectCfg{legacyRoot},
		},
	}
	err := pullHandler.Handle(context.Background(), &http.BufferedRequest{}, event.PullRequest{})
	assert.NoError(t, err)
	assert.True(t, workerProxy.called)
}

func TestModifiedPullHandler_Handle_BranchStrategy(t *testing.T) {
	testRepo := models.Repo{
		FullName: "owner/repo",
		VCSHost: models.VCSHost{
			Hostname: "github.com",
		},
	}
	pullRequest := models.PullRequest{
		HeadCommit: "sha",
		HeadBranch: "branch",
		BaseBranch: "main",
		BaseRepo:   testRepo,
		HeadRepo:   testRepo,
	}
	logger := logging.NewNoopCtxLogger(t)
	statusUpdater := &mockVCSStatusUpdater{
		combinedCountCalls: 1,
	}
	legacyRoot := &valid.MergedProjectCfg{
		Name:         "legacy",
		WorkflowMode: valid.DefaultWorkflowMode,
	}
	globalCfg := valid.GlobalCfg{
		Repos: []valid.Repo{
			{
				ID:               "github.com/owner/repo",
				CheckoutStrategy: "branch",
			},
		},
	}
	workerProxy := &mockWorkerProxy{}
	expectedCommit := &config.RepoCommit{
		Repo:   testRepo,
		Branch: "branch",
		Sha:    "sha",
	}
	pullHandler := event.ModifiedPullHandler{
		Logger:             logger,
		Scheduler:          &sync.SynchronousScheduler{Logger: logger},
		GlobalCfg:          globalCfg,
		RequirementChecker: &requirementsChecker{},
		VCSStatusUpdater:   statusUpdater,
		WorkerProxy:        workerProxy,
		RootConfigBuilder: &mockConfigBuilder{
			expectedCommit:     expectedCommit,
			expectedCloneDepth: 1,
			expectedT:          t,
			rootConfigs:        []*valid.MergedProjectCfg{legacyRoot},
		},
	}
	err := pullHandler.Handle(context.Background(), &http.BufferedRequest{}, event.PullRequest{
		Pull: pullRequest,
	})
	assert.NoError(t, err)
	assert.True(t, workerProxy.called)
}

type mockConfigBuilder struct {
	expectedCommit     *config.RepoCommit
	expectedToken      int64
	expectedCloneDepth int
	expectedT          *testing.T
	rootConfigs        []*valid.MergedProjectCfg
	error              error
}

func (r *mockConfigBuilder) Build(_ context.Context, commit *config.RepoCommit, installationToken int64, opts ...config.BuilderOptions) ([]*valid.MergedProjectCfg, error) {
	assert.Equal(r.expectedT, r.expectedCommit, commit)
	assert.Equal(r.expectedT, r.expectedToken, installationToken)
	assert.Len(r.expectedT, opts, 1)
	assert.Equal(r.expectedT, r.expectedCloneDepth, opts[0].RepoFetcherOptions.CloneDepth)
	return r.rootConfigs, r.error
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
