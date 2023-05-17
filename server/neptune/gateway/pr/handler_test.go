package pr_test

import (
	"context"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/neptune/gateway/config"
	"github.com/runatlantis/atlantis/server/neptune/gateway/pr"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/client"
	"testing"
)

func TestRevisionHandler_Handle_NoRoots(t *testing.T) {
	token := int64(123)
	prNum := 1
	branch := "branch"
	revision := "abc"
	testRepo := models.Repo{
		FullName: "owner/repo",
	}
	commit := &config.RepoCommit{
		Repo:          testRepo,
		Branch:        branch,
		Sha:           revision,
		OptionalPRNum: prNum,
	}
	signaler := &mockSignaler{
		t: t,
	}
	builder := &mockRootConfigBuilder{
		expectedCommit:     commit,
		expectedToken:      token,
		expectedCloneDepth: -1,
		expectedT:          t,
	}
	globalCfg := valid.GlobalCfg{}
	handler := pr.RevisionHandler{
		Logger:            logging.NewNoopCtxLogger(t),
		GlobalCfg:         globalCfg,
		RootConfigBuilder: builder,
		PRSignaler:        signaler,
	}
	prOptions := pr.Options{
		Number:            prNum,
		Revision:          revision,
		Repo:              testRepo,
		InstallationToken: token,
		Branch:            branch,
	}
	err := handler.Handle(context.Background(), prOptions)
	assert.True(t, builder.called)
	assert.False(t, signaler.called)
	assert.NoError(t, err)
}

func TestRevisionHandler_Handle_RootFailure(t *testing.T) {
	token := int64(123)
	prNum := 1
	branch := "branch"
	revision := "abc"
	testRepo := models.Repo{
		FullName: "owner/repo",
	}
	commit := &config.RepoCommit{
		Repo:          testRepo,
		Branch:        branch,
		Sha:           revision,
		OptionalPRNum: prNum,
	}
	signaler := &mockSignaler{
		t: t,
	}
	builder := &mockRootConfigBuilder{
		expectedCommit:     commit,
		expectedToken:      token,
		expectedCloneDepth: -1,
		expectedT:          t,
		error:              assert.AnError,
	}
	globalCfg := valid.GlobalCfg{}
	handler := pr.RevisionHandler{
		Logger:            logging.NewNoopCtxLogger(t),
		GlobalCfg:         globalCfg,
		RootConfigBuilder: builder,
		PRSignaler:        signaler,
	}
	prOptions := pr.Options{
		Number:            prNum,
		Revision:          revision,
		Repo:              testRepo,
		InstallationToken: token,
		Branch:            branch,
	}
	err := handler.Handle(context.Background(), prOptions)
	assert.False(t, signaler.called)
	assert.Error(t, err)
}

func TestRevisionHandler_Handle_Branch(t *testing.T) {
	token := int64(123)
	prNum := 1
	branch := "branch"
	revision := "abc"
	testRepo := models.Repo{
		FullName: "owner/repo",
		VCSHost: models.VCSHost{
			Hostname: "github.com",
		},
	}
	commit := &config.RepoCommit{
		Repo:          testRepo,
		Branch:        branch,
		Sha:           revision,
		OptionalPRNum: prNum,
	}
	signaler := &mockSignaler{
		t: t,
	}
	builder := &mockRootConfigBuilder{
		expectedCommit:     commit,
		expectedToken:      token,
		expectedCloneDepth: 1,
		expectedT:          t,
	}
	globalCfg := valid.GlobalCfg{
		Repos: []valid.Repo{
			{
				ID:               "github.com/owner/repo",
				CheckoutStrategy: "branch",
			},
		},
	}
	handler := pr.RevisionHandler{
		Logger:            logging.NewNoopCtxLogger(t),
		GlobalCfg:         globalCfg,
		RootConfigBuilder: builder,
		PRSignaler:        signaler,
	}
	prOptions := pr.Options{
		Number:            prNum,
		Revision:          revision,
		Repo:              testRepo,
		InstallationToken: token,
		Branch:            branch,
	}
	err := handler.Handle(context.Background(), prOptions)
	assert.False(t, signaler.called)
	assert.NoError(t, err)
}

func TestRevisionHandler_Handle_Success(t *testing.T) {
	token := int64(123)
	prNum := 1
	branch := "branch"
	revision := "abc"
	testRepo := models.Repo{
		FullName: "owner/repo",
	}
	commit := &config.RepoCommit{
		Repo:          testRepo,
		Branch:        branch,
		Sha:           revision,
		OptionalPRNum: prNum,
	}
	legacyRoot := &valid.MergedProjectCfg{
		Name:         "legacy",
		WorkflowMode: valid.DefaultWorkflowMode,
	}
	platformRoot := &valid.MergedProjectCfg{
		Name:         "platform",
		WorkflowMode: valid.PlatformWorkflowMode,
	}
	builder := &mockRootConfigBuilder{
		expectedCommit:     commit,
		expectedToken:      token,
		expectedCloneDepth: -1,
		expectedT:          t,
		rootConfigs:        []*valid.MergedProjectCfg{legacyRoot, platformRoot},
	}
	prOptions := pr.Options{
		Number:            prNum,
		Revision:          revision,
		Repo:              testRepo,
		InstallationToken: token,
		Branch:            branch,
	}
	globalCfg := valid.GlobalCfg{}
	signaler := &mockSignaler{
		expectedRoots:   []*valid.MergedProjectCfg{platformRoot},
		expectedOptions: prOptions,
		t:               t,
	}
	handler := pr.RevisionHandler{
		Logger:            logging.NewNoopCtxLogger(t),
		GlobalCfg:         globalCfg,
		RootConfigBuilder: builder,
		PRSignaler:        signaler,
	}
	err := handler.Handle(context.Background(), prOptions)
	assert.True(t, builder.called)
	assert.True(t, signaler.called)
	assert.NoError(t, err)
}

func TestRevisionHandler_Handle_SignalFailure(t *testing.T) {
	token := int64(123)
	prNum := 1
	branch := "branch"
	revision := "abc"
	testRepo := models.Repo{
		FullName: "owner/repo",
	}
	commit := &config.RepoCommit{
		Repo:          testRepo,
		Branch:        branch,
		Sha:           revision,
		OptionalPRNum: prNum,
	}
	platformRoot := &valid.MergedProjectCfg{
		Name:         "platform",
		WorkflowMode: valid.PlatformWorkflowMode,
	}
	builder := &mockRootConfigBuilder{
		expectedCommit:     commit,
		expectedToken:      token,
		expectedCloneDepth: -1,
		expectedT:          t,
		rootConfigs:        []*valid.MergedProjectCfg{platformRoot},
	}
	prOptions := pr.Options{
		Number:            prNum,
		Revision:          revision,
		Repo:              testRepo,
		InstallationToken: token,
		Branch:            branch,
	}
	globalCfg := valid.GlobalCfg{}
	signaler := &mockSignaler{
		expectedRoots:   []*valid.MergedProjectCfg{platformRoot},
		expectedOptions: prOptions,
		error:           assert.AnError,
		t:               t,
	}
	handler := pr.RevisionHandler{
		Logger:            logging.NewNoopCtxLogger(t),
		GlobalCfg:         globalCfg,
		RootConfigBuilder: builder,
		PRSignaler:        signaler,
	}
	err := handler.Handle(context.Background(), prOptions)
	assert.True(t, builder.called)
	assert.True(t, signaler.called)
	assert.Error(t, err)
}

type mockRootConfigBuilder struct {
	expectedCommit     *config.RepoCommit
	expectedToken      int64
	expectedCloneDepth int
	expectedT          *testing.T
	called             bool
	rootConfigs        []*valid.MergedProjectCfg
	error              error
}

func (r *mockRootConfigBuilder) Build(_ context.Context, commit *config.RepoCommit, installationToken int64, opts ...config.BuilderOptions) ([]*valid.MergedProjectCfg, error) {
	r.called = true
	assert.Equal(r.expectedT, r.expectedCommit, commit)
	assert.Equal(r.expectedT, r.expectedToken, installationToken)
	assert.Len(r.expectedT, opts, 1)
	assert.Equal(r.expectedT, r.expectedCloneDepth, opts[0].RepoFetcherOptions.CloneDepth)
	return r.rootConfigs, r.error
}

type mockSignaler struct {
	expectedRoots   []*valid.MergedProjectCfg
	expectedOptions pr.Options
	error           error
	called          bool
	t               assert.TestingT
}

func (s *mockSignaler) SignalWithStartWorkflow(_ context.Context, roots []*valid.MergedProjectCfg, options pr.Options) (client.WorkflowRun, error) {
	s.called = true
	assert.Equal(s.t, s.expectedRoots, roots)
	assert.Equal(s.t, s.expectedOptions, options)
	if s.error != nil {
		return nil, s.error
	}
	return testRun{}, nil
}
