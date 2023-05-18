package event_test

import (
	"context"
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
	expectedCommit := &config.RepoCommit{
		Repo:   testRepo,
		Branch: "branch",
		Sha:    "sha",
	}
	pr := event.PullRequest{
		Pull: pullRequest,
	}
	legacyHandler := &mockLegacyHandler{
		expectedEvent:       pr,
		expectedAllRoots:    []*valid.MergedProjectCfg{legacyRoot},
		expectedLegacyRoots: []*valid.MergedProjectCfg{legacyRoot},
		expectedT:           t,
	}
	pullHandler := event.ModifiedPullHandler{
		Logger:             logger,
		Scheduler:          &sync.SynchronousScheduler{Logger: logger},
		GlobalCfg:          globalCfg,
		RequirementChecker: &requirementsChecker{},
		RootConfigBuilder: &mockConfigBuilder{
			expectedCommit:     expectedCommit,
			expectedCloneDepth: 1,
			expectedT:          t,
			rootConfigs:        []*valid.MergedProjectCfg{legacyRoot},
		},
		LegacyHandler: legacyHandler,
	}
	err := pullHandler.Handle(context.Background(), &http.BufferedRequest{}, pr)
	assert.NoError(t, err)
	assert.True(t, legacyHandler.called)
}

func TestModifiedPullHandler_Handle_MergeStrategy(t *testing.T) {
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
	legacyRoot := &valid.MergedProjectCfg{
		Name:         "legacy",
		WorkflowMode: valid.DefaultWorkflowMode,
	}
	globalCfg := valid.GlobalCfg{}
	expectedCommit := &config.RepoCommit{
		Repo:   testRepo,
		Branch: "branch",
		Sha:    "sha",
	}
	pr := event.PullRequest{
		Pull: pullRequest,
	}
	legacyHandler := &mockLegacyHandler{
		expectedEvent:       pr,
		expectedAllRoots:    []*valid.MergedProjectCfg{legacyRoot},
		expectedLegacyRoots: []*valid.MergedProjectCfg{legacyRoot},
		expectedT:           t,
	}
	pullHandler := event.ModifiedPullHandler{
		Logger:             logger,
		Scheduler:          &sync.SynchronousScheduler{Logger: logger},
		GlobalCfg:          globalCfg,
		RequirementChecker: &requirementsChecker{},
		RootConfigBuilder: &mockConfigBuilder{
			expectedCommit: expectedCommit,
			expectedT:      t,
			rootConfigs:    []*valid.MergedProjectCfg{legacyRoot},
		},
		LegacyHandler: legacyHandler,
	}
	err := pullHandler.Handle(context.Background(), &http.BufferedRequest{}, pr)
	assert.NoError(t, err)
	assert.True(t, legacyHandler.called)
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

type mockLegacyHandler struct {
	expectedEvent       event.PullRequest
	expectedAllRoots    []*valid.MergedProjectCfg
	expectedLegacyRoots []*valid.MergedProjectCfg
	expectedT           *testing.T
	error               error
	called              bool
}

func (l *mockLegacyHandler) Handle(ctx context.Context, _ *http.BufferedRequest, event event.PullRequest, allRoots []*valid.MergedProjectCfg, legacyRoots []*valid.MergedProjectCfg) error {
	l.called = true
	assert.Equal(l.expectedT, l.expectedEvent, event)
	assert.Equal(l.expectedT, l.expectedAllRoots, allRoots)
	assert.Equal(l.expectedT, l.expectedLegacyRoots, legacyRoots)
	return l.error
}
