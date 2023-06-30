package event_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/legacy/events/command"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/models"
	"github.com/runatlantis/atlantis/server/neptune/gateway/config"
	"github.com/runatlantis/atlantis/server/neptune/gateway/deploy"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event"
	"github.com/runatlantis/atlantis/server/neptune/gateway/pr"
	"github.com/runatlantis/atlantis/server/neptune/gateway/requirement"
	"github.com/runatlantis/atlantis/server/neptune/sync"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/client"
)

var testRepo = models.Repo{
	FullName: repoFullName,
}
var testPull = models.PullRequest{
	BaseRepo:   testRepo,
	HeadBranch: "somebranch",
	HeadCommit: "1234",
	Num:        1,
}

type noopErrorHandler struct{}

func (h noopErrorHandler) WrapWithHandling(ctx context.Context, event event.PREvent, commandName string, executor sync.Executor) sync.Executor {
	return executor
}

type requirementsChecker struct {
	err error
}

func (a *requirementsChecker) Check(ctx context.Context, criteria requirement.Criteria) error {
	return a.err
}

type mockRootConfigBuilder struct {
	expectedCommit  *config.RepoCommit
	expectedToken   int64
	expectedOptions []config.BuilderOptions
	expectedT       *testing.T
	rootConfigs     []*valid.MergedProjectCfg
	error           error
}

func (r *mockRootConfigBuilder) Build(_ context.Context, commit *config.RepoCommit, installationToken int64, opts ...config.BuilderOptions) ([]*valid.MergedProjectCfg, error) {
	assert.Equal(r.expectedT, r.expectedCommit, commit)
	assert.Equal(r.expectedT, r.expectedToken, installationToken)
	assert.Equal(r.expectedT, r.expectedOptions, opts)
	return r.rootConfigs, r.error
}

type testMultiDeploySignaler struct {
	signalers []*testDeploySignaler
	count     int
}

func (d *testMultiDeploySignaler) SignalWorkflow(_ context.Context, _ string, _ string, _ string, _ interface{}) error {
	return nil
}

func (d *testMultiDeploySignaler) SignalWithStartWorkflow(ctx context.Context, cfg *valid.MergedProjectCfg, opts deploy.RootDeployOptions) (client.WorkflowRun, error) {
	if d.count >= len(d.signalers) {
		panic(nil)
	}

	r, e := d.signalers[d.count].SignalWithStartWorkflow(ctx, cfg, opts)
	d.count++

	return r, e
}

func (d *testMultiDeploySignaler) called() bool {
	return d.count == len(d.signalers)
}

type testDeploySignaler struct {
	expectedT   *testing.T
	called      bool
	expectedCfg *valid.MergedProjectCfg
	expOpts     deploy.RootDeployOptions
}

func (d *testDeploySignaler) SignalWorkflow(_ context.Context, _ string, _ string, _ string, _ interface{}) error {
	d.called = true
	return nil
}

func (d *testDeploySignaler) SignalWithStartWorkflow(ctx context.Context, cfg *valid.MergedProjectCfg, opts deploy.RootDeployOptions) (client.WorkflowRun, error) {
	assert.Equal(d.expectedT, d.expectedCfg, cfg)
	assert.Equal(d.expectedT, d.expOpts, opts)
	d.called = true

	return nil, nil
}

func TestCommentEventWorkerProxy_HandleForceApply(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	commentEvent := event.Comment{
		Pull:     testPull,
		PullNum:  testPull.Num,
		BaseRepo: testRepo,
		HeadRepo: testRepo,
		User: models.User{
			Username: "someuser",
		},
		InstallationToken: 123,
	}
	expectedOpts := deploy.RootDeployOptions{
		Repo:              testRepo,
		Branch:            testPull.HeadBranch,
		Revision:          testPull.HeadCommit,
		OptionalPullNum:   testPull.Num,
		Sender:            commentEvent.User,
		InstallationToken: commentEvent.InstallationToken,
		TriggerInfo: workflows.DeployTriggerInfo{
			Type:  workflows.ManualTrigger,
			Force: true,
		},
	}
	rootConfigBuilder := &mockRootConfigBuilder{
		expectedT: t,
		expectedCommit: &config.RepoCommit{
			Repo:          testRepo,
			Branch:        testPull.HeadBranch,
			Sha:           testPull.HeadCommit,
			OptionalPRNum: testPull.Num,
		},
		expectedToken: 123,
		rootConfigs: []*valid.MergedProjectCfg{
			{
				Name: "root1",
			},
			{
				Name: "root2",
			},
		},
	}
	testSignaler := &testMultiDeploySignaler{
		signalers: []*testDeploySignaler{
			{
				expectedT:   t,
				expectedCfg: rootConfigBuilder.rootConfigs[0],
				expOpts:     expectedOpts,
			},
			{
				expectedT:   t,
				expectedCfg: rootConfigBuilder.rootConfigs[1],
				expOpts:     expectedOpts,
			},
		},
	}
	writer := &mockSnsWriter{}
	scheduler := &sync.SynchronousScheduler{Logger: logger}
	commentCreator := &mockCommentCreator{
		expectedT:       t,
		expectedRepo:    testRepo,
		expectedPull:    testPull.Num,
		expectedMessage: "⚠️ WARNING ⚠️\n\n You are force applying changes from your PR instead of merging into your default branch 🚀. This can have unpredictable consequences 🙏🏽 and should only be used in an emergency 🆘.\n\n To confirm behavior, review and confirm the plan within the generated atlantis/deploy GH check below.\n\n 𝐓𝐡𝐢𝐬 𝐚𝐜𝐭𝐢𝐨𝐧 𝐰𝐢𝐥𝐥 𝐛𝐞 𝐚𝐮𝐝𝐢𝐭𝐞𝐝.\n",
	}
	statusUpdater := &mockStatusUpdater{}
	cfg := valid.NewGlobalCfg("somedir")
	prSignaler := &mockPRSignaler{
		expectedT: t,
	}
	commentEventWorkerProxy := event.NewCommentEventWorkerProxy(logger, writer, scheduler, prSignaler, testSignaler, commentCreator, statusUpdater, cfg, rootConfigBuilder, noopErrorHandler{}, noopErrorHandler{}, &requirementsChecker{})
	bufReq := buildRequest(t)
	cmd := &command.Comment{
		Name:       command.Apply,
		ForceApply: true,
	}
	err := commentEventWorkerProxy.Handle(context.Background(), bufReq, commentEvent, cmd)
	assert.NoError(t, err)
	assert.True(t, commentCreator.isCalled)
	assert.True(t, testSignaler.called())
	assert.False(t, writer.isCalled)
	assert.False(t, statusUpdater.isCalled)
}

func TestCommentEventWorkerProxy_HandleApplyComment_RequirementsFailed(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	rootConfigBuilder := &mockRootConfigBuilder{
		expectedT: t,
		expectedCommit: &config.RepoCommit{
			Repo:          testRepo,
			Branch:        testPull.HeadBranch,
			Sha:           testPull.HeadCommit,
			OptionalPRNum: testPull.Num,
		},
		expectedToken: 123,
		rootConfigs: []*valid.MergedProjectCfg{
			{
				Name: "root1",
			},
			{
				Name: "root2",
			},
		},
	}
	commentEvent := event.Comment{
		Pull:     testPull,
		PullNum:  testPull.Num,
		BaseRepo: testRepo,
		HeadRepo: testRepo,
		User: models.User{
			Username: "someuser",
		},
		InstallationToken: 123,
	}
	testSignaler := &testDeploySignaler{}
	prSignaler := &mockPRSignaler{
		expectedT: t,
	}

	writer := &mockSnsWriter{}
	scheduler := &sync.SynchronousScheduler{Logger: logger}
	commentCreator := &mockCommentCreator{}
	statusUpdater := &mockStatusUpdater{}
	cfg := valid.NewGlobalCfg("somedir")
	commentEventWorkerProxy := event.NewCommentEventWorkerProxy(logger, writer, scheduler, prSignaler, testSignaler, commentCreator, statusUpdater, cfg, rootConfigBuilder, noopErrorHandler{}, noopErrorHandler{}, &requirementsChecker{
		err: assert.AnError,
	})
	bufReq := buildRequest(t)
	cmd := &command.Comment{
		Name: command.Apply,
	}
	err := commentEventWorkerProxy.Handle(context.Background(), bufReq, commentEvent, cmd)
	assert.Error(t, err)
	assert.False(t, statusUpdater.isCalled)
	assert.False(t, commentCreator.isCalled)
	assert.False(t, testSignaler.called)
	assert.False(t, writer.isCalled)
}

func TestCommentEventWorkerProxy_HandleApplyComment(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	rootConfigBuilder := &mockRootConfigBuilder{
		expectedT: t,
		expectedCommit: &config.RepoCommit{
			Repo:          testRepo,
			Branch:        testPull.HeadBranch,
			Sha:           testPull.HeadCommit,
			OptionalPRNum: testPull.Num,
		},
		expectedToken: 123,
		rootConfigs: []*valid.MergedProjectCfg{
			{
				Name: "root1",
			},
			{
				Name: "root2",
			},
		},
	}
	commentEvent := event.Comment{
		Pull:     testPull,
		PullNum:  testPull.Num,
		BaseRepo: testRepo,
		HeadRepo: testRepo,
		User: models.User{
			Username: "someuser",
		},
		InstallationToken: 123,
	}
	expectedOpts := deploy.RootDeployOptions{
		Repo:              testRepo,
		Branch:            testPull.HeadBranch,
		Revision:          testPull.HeadCommit,
		OptionalPullNum:   testPull.Num,
		Sender:            commentEvent.User,
		InstallationToken: commentEvent.InstallationToken,
		TriggerInfo: workflows.DeployTriggerInfo{
			Type: workflows.ManualTrigger,
		},
	}
	testSignaler := &testMultiDeploySignaler{
		signalers: []*testDeploySignaler{
			{
				expectedT:   t,
				expectedCfg: rootConfigBuilder.rootConfigs[0],
				expOpts:     expectedOpts,
			},
			{
				expectedT:   t,
				expectedCfg: rootConfigBuilder.rootConfigs[1],
				expOpts:     expectedOpts,
			},
		},
	}

	writer := &mockSnsWriter{}
	scheduler := &sync.SynchronousScheduler{Logger: logger}
	commentCreator := &mockCommentCreator{}
	statusUpdater := &mockStatusUpdater{}
	cfg := valid.NewGlobalCfg("somedir")
	prSignaler := &mockPRSignaler{
		expectedT: t,
	}
	commentEventWorkerProxy := event.NewCommentEventWorkerProxy(logger, writer, scheduler, prSignaler, testSignaler, commentCreator, statusUpdater, cfg, rootConfigBuilder, noopErrorHandler{}, noopErrorHandler{}, &requirementsChecker{})
	bufReq := buildRequest(t)
	cmd := &command.Comment{
		Name: command.Apply,
	}
	err := commentEventWorkerProxy.Handle(context.Background(), bufReq, commentEvent, cmd)
	assert.NoError(t, err)
	assert.False(t, statusUpdater.isCalled)
	assert.False(t, commentCreator.isCalled)
	assert.True(t, testSignaler.called())
	assert.False(t, writer.isCalled)
}

func TestCommentEventWorkerProxy_HandlePlanComment_NoCmds(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	rootConfigBuilder := &mockRootConfigBuilder{
		expectedT: t,
		expectedCommit: &config.RepoCommit{
			Repo:          testRepo,
			Branch:        testPull.HeadBranch,
			Sha:           testPull.HeadCommit,
			OptionalPRNum: testPull.Num,
		},
		expectedToken: 123,
	}
	testSignaler := &testDeploySignaler{}
	commentEvent := event.Comment{
		Pull:     testPull,
		PullNum:  testPull.Num,
		BaseRepo: testRepo,
		HeadRepo: testRepo,
		User: models.User{
			Username: "someuser",
		},
		InstallationToken: 123,
	}
	writer := &mockSnsWriter{}
	scheduler := &sync.SynchronousScheduler{Logger: logger}
	commentCreator := &mockCommentCreator{}
	statusUpdater := &multiMockStatusUpdater{
		delegates: []*mockStatusUpdater{
			{
				expectedRepo:      testRepo,
				expectedPull:      testPull,
				expectedVCSStatus: models.SuccessVCSStatus,
				expectedCmd:       command.Plan.String(),
				expectedBody:      "no modified roots",
				expectedT:         t,
			},
			{
				expectedRepo:      testRepo,
				expectedPull:      testPull,
				expectedVCSStatus: models.SuccessVCSStatus,
				expectedCmd:       command.PolicyCheck.String(),
				expectedBody:      "no modified roots",
				expectedT:         t,
			},
			{
				expectedRepo:      testRepo,
				expectedPull:      testPull,
				expectedVCSStatus: models.SuccessVCSStatus,
				expectedCmd:       command.Apply.String(),
				expectedBody:      "no modified roots",
				expectedT:         t,
			},
		},
	}
	cfg := valid.NewGlobalCfg("somedir")
	prSignaler := &mockPRSignaler{
		expectedT: t,
	}
	commentEventWorkerProxy := event.NewCommentEventWorkerProxy(logger, writer, scheduler, prSignaler, testSignaler, commentCreator, statusUpdater, cfg, rootConfigBuilder, noopErrorHandler{}, noopErrorHandler{}, &requirementsChecker{})
	bufReq := buildRequest(t)
	cmd := &command.Comment{
		Name: command.Plan,
	}
	err := commentEventWorkerProxy.Handle(context.Background(), bufReq, commentEvent, cmd)
	assert.NoError(t, err)
	assert.True(t, statusUpdater.AllCalled())
	assert.False(t, commentCreator.isCalled)
	assert.False(t, testSignaler.called)
	assert.False(t, writer.isCalled)
}

func TestCommentEventWorkerProxy_HandleApplyComment_NoCmds(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	rootConfigBuilder := &mockRootConfigBuilder{
		expectedT: t,
		expectedCommit: &config.RepoCommit{
			Repo:          testRepo,
			Branch:        testPull.HeadBranch,
			Sha:           testPull.HeadCommit,
			OptionalPRNum: testPull.Num,
		},
		expectedToken: 123,
	}
	testSignaler := &testDeploySignaler{}
	commentEvent := event.Comment{
		Pull:     testPull,
		PullNum:  testPull.Num,
		BaseRepo: testRepo,
		HeadRepo: testRepo,
		User: models.User{
			Username: "someuser",
		},
		InstallationToken: 123,
	}
	writer := &mockSnsWriter{}
	scheduler := &sync.SynchronousScheduler{Logger: logger}
	commentCreator := &mockCommentCreator{}
	statusUpdater := &multiMockStatusUpdater{
		delegates: []*mockStatusUpdater{
			{
				expectedRepo:      testRepo,
				expectedPull:      testPull,
				expectedVCSStatus: models.SuccessVCSStatus,
				expectedCmd:       command.Apply.String(),
				expectedBody:      "no modified roots",
				expectedT:         t,
			},
		},
	}
	cfg := valid.NewGlobalCfg("somedir")
	prSignaler := &mockPRSignaler{
		expectedT: t,
	}
	commentEventWorkerProxy := event.NewCommentEventWorkerProxy(logger, writer, scheduler, prSignaler, testSignaler, commentCreator, statusUpdater, cfg, rootConfigBuilder, noopErrorHandler{}, noopErrorHandler{}, &requirementsChecker{})
	bufReq := buildRequest(t)
	cmd := &command.Comment{
		Name: command.Apply,
	}
	err := commentEventWorkerProxy.Handle(context.Background(), bufReq, commentEvent, cmd)
	assert.NoError(t, err)
	assert.True(t, statusUpdater.AllCalled())
	assert.False(t, commentCreator.isCalled)
	assert.False(t, testSignaler.called)
	assert.False(t, writer.isCalled)
}

func TestCommentEventWorkerProxy_HandlePlanComment(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	roots := []*valid.MergedProjectCfg{
		{
			Name: "root2",
		},
	}
	token := int64(123)
	prRequest := pr.Request{
		Number:            testPull.Num,
		Revision:          testPull.HeadCommit,
		Repo:              testPull.HeadRepo,
		InstallationToken: token,
		Branch:            testPull.HeadBranch,
		ValidateEnvs: []pr.ValidateEnvs{
			{
				PullNum:    testPull.Num,
				PullAuthor: testPull.Author,
				HeadCommit: testPull.HeadCommit,
				Username:   "someuser",
			},
		},
	}
	rootConfigBuilder := &mockRootConfigBuilder{
		expectedT: t,
		expectedCommit: &config.RepoCommit{
			Repo:          testRepo,
			Branch:        testPull.HeadBranch,
			Sha:           testPull.HeadCommit,
			OptionalPRNum: testPull.Num,
		},
		expectedToken: 123,
		rootConfigs:   roots,
	}
	deploySignaler := &testDeploySignaler{}
	commentEvent := event.Comment{
		Pull:     testPull,
		PullNum:  testPull.Num,
		BaseRepo: testRepo,
		HeadRepo: testRepo,
		User: models.User{
			Username: "someuser",
		},
		InstallationToken: 123,
	}
	writer := &mockSnsWriter{}
	scheduler := &sync.SynchronousScheduler{Logger: logger}
	commentCreator := &mockCommentCreator{}
	statusUpdater := &mockStatusUpdater{
		expectedRepo:      testRepo,
		expectedPull:      testPull,
		expectedVCSStatus: models.QueuedVCSStatus,
		expectedCmd:       command.Plan.String(),
		expectedBody:      "Request received. Adding to the queue...",
		expectedT:         t,
	}
	cfg := valid.NewGlobalCfg("somedir")
	prSignaler := &mockPRSignaler{
		expectedT:         t,
		expectedRoots:     roots,
		expectedPRRequest: prRequest,
	}
	commentEventWorkerProxy := event.NewCommentEventWorkerProxy(logger, writer, scheduler, prSignaler, deploySignaler, commentCreator, statusUpdater, cfg, rootConfigBuilder, noopErrorHandler{}, noopErrorHandler{}, &requirementsChecker{})
	bufReq := buildRequest(t)
	cmd := &command.Comment{
		Name: command.Plan,
	}
	err := commentEventWorkerProxy.Handle(context.Background(), bufReq, commentEvent, cmd)
	assert.NoError(t, err)
	assert.True(t, statusUpdater.isCalled)
	assert.False(t, commentCreator.isCalled)
	assert.False(t, deploySignaler.called)
	assert.True(t, prSignaler.called)
	assert.True(t, writer.isCalled)
}

func TestCommentEventWorkerProxy_HandlePlanCommentAllocatorEnabled(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	roots := []*valid.MergedProjectCfg{
		{
			Name: "root2",
		},
	}
	token := int64(123)
	prRequest := pr.Request{
		Number:            testPull.Num,
		Revision:          testPull.HeadCommit,
		Repo:              testPull.HeadRepo,
		InstallationToken: token,
		Branch:            testPull.HeadBranch,
		ValidateEnvs: []pr.ValidateEnvs{
			{
				PullNum:    testPull.Num,
				PullAuthor: testPull.Author,
				HeadCommit: testPull.HeadCommit,
				Username:   "someuser",
			},
		},
	}
	rootConfigBuilder := &mockRootConfigBuilder{
		expectedT: t,
		expectedCommit: &config.RepoCommit{
			Repo:          testRepo,
			Branch:        testPull.HeadBranch,
			Sha:           testPull.HeadCommit,
			OptionalPRNum: testPull.Num,
		},
		expectedToken: token,
		rootConfigs:   roots,
	}
	testSignaler := &testDeploySignaler{}
	commentEvent := event.Comment{
		Pull:     testPull,
		PullNum:  testPull.Num,
		BaseRepo: testRepo,
		HeadRepo: testRepo,
		User: models.User{
			Username: "someuser",
		},
		InstallationToken: 123,
	}
	writer := &mockSnsWriter{}
	scheduler := &sync.SynchronousScheduler{Logger: logger}
	commentCreator := &mockCommentCreator{}
	statusUpdater := &mockStatusUpdater{
		expectedRepo:      testRepo,
		expectedPull:      testPull,
		expectedVCSStatus: models.QueuedVCSStatus,
		expectedCmd:       command.Plan.String(),
		expectedBody:      "Request received. Adding to the queue...",
		expectedT:         t,
	}
	cfg := valid.NewGlobalCfg("somedir")
	prSignaler := &mockPRSignaler{
		expectedT:         t,
		expectedRoots:     roots,
		expectedPRRequest: prRequest,
	}
	commentEventWorkerProxy := event.NewCommentEventWorkerProxy(logger, writer, scheduler, prSignaler, testSignaler, commentCreator, statusUpdater, cfg, rootConfigBuilder, noopErrorHandler{}, noopErrorHandler{}, &requirementsChecker{})
	bufReq := buildRequest(t)
	cmd := &command.Comment{
		Name: command.Plan,
	}
	err := commentEventWorkerProxy.Handle(context.Background(), bufReq, commentEvent, cmd)
	assert.NoError(t, err)
	assert.True(t, statusUpdater.isCalled)
	assert.False(t, commentCreator.isCalled)
	assert.False(t, testSignaler.called)
	assert.True(t, writer.isCalled)
	assert.True(t, prSignaler.called)
}

type mockCommentCreator struct {
	isCalled        bool
	expectedT       *testing.T
	expectedRepo    models.Repo
	expectedPull    int
	expectedMessage string
	err             error
}

func (c *mockCommentCreator) CreateComment(repo models.Repo, pull int, message string, _ string) error {
	c.isCalled = true
	assert.Equal(c.expectedT, c.expectedRepo, repo)
	assert.Equal(c.expectedT, c.expectedPull, pull)
	assert.Equal(c.expectedT, c.expectedMessage, message)

	return c.err
}

type multiMockStatusUpdater struct {
	delegates []*mockStatusUpdater
	index     int
}

func (s *multiMockStatusUpdater) AllCalled() bool {
	for _, d := range s.delegates {
		if !d.isCalled {
			return false
		}
	}

	return true
}

func (s *multiMockStatusUpdater) UpdateCombined(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.VCSStatus, cmd fmt.Stringer, ss string, body string) (string, error) {
	if s.index >= len(s.delegates) {
		panic(nil)
	}

	result, err := s.delegates[s.index].UpdateCombined(ctx, repo, pull, status, cmd, ss, body)
	s.index++

	return result, err
}

type mockStatusUpdater struct {
	isCalled          bool
	expectedRepo      models.Repo
	expectedPull      models.PullRequest
	expectedVCSStatus models.VCSStatus
	expectedCmd       string
	expectedBody      string
	expectedT         *testing.T
	err               error
}

func (s *mockStatusUpdater) UpdateCombined(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.VCSStatus, cmd fmt.Stringer, _ string, body string) (string, error) {
	s.isCalled = true

	assert.Equal(s.expectedT, s.expectedRepo, repo)
	assert.Equal(s.expectedT, s.expectedPull, pull)
	assert.Equal(s.expectedT, s.expectedVCSStatus, status)
	assert.Equal(s.expectedT, s.expectedCmd, cmd.String())
	assert.Equal(s.expectedT, s.expectedBody, body)

	return "", s.err
}
