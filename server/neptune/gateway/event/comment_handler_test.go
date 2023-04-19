package event_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/lyft/feature"
	"github.com/runatlantis/atlantis/server/metrics"
	"github.com/runatlantis/atlantis/server/neptune/gateway/deploy"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event"
	"github.com/runatlantis/atlantis/server/neptune/sync"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"github.com/stretchr/testify/assert"
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

type testProjectCommandGetter struct {
	expectedProjectContext []command.ProjectContext
}

func (g *testProjectCommandGetter) GetProjectCommands(cmdCtx *command.Context, baseRepo models.Repo, pull models.PullRequest) ([]command.ProjectContext, error) {
	return g.expectedProjectContext, nil
}

func TestCommentEventWorkerProxy_HandleAllocationError(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	scope, _, err := metrics.NewLoggingScope(logger, "")
	assert.NoError(t, err)
	getter := &testProjectCommandGetter{}
	writer := &mockSnsWriter{}
	allocator := &testAllocator{
		t:                 t,
		expectedFeatureID: feature.PlatformMode,
		expectedFeatureCtx: feature.FeatureContext{
			RepoName: repoFullName,
		},
		expectedError: assert.AnError,
	}
	scheduler := &sync.SynchronousScheduler{Logger: logger}
	rootDeployer := &mockRootDeployer{}
	commentCreator := &mockCommentCreator{}
	statusUpdater := &mockStatusUpdater{}
	cfg := valid.NewGlobalCfg("somedir")
	commentEventWorkerProxy := event.NewCommentEventWorkerProxy(logger, scope, writer, allocator, scheduler, rootDeployer, commentCreator, statusUpdater, cfg, getter)
	bufReq := buildRequest(t)
	commentEvent := event.Comment{
		Pull:     testPull,
		BaseRepo: testRepo,
	}
	cmd := &command.Comment{
		Name: command.Plan,
	}
	err = commentEventWorkerProxy.Handle(context.Background(), bufReq, commentEvent, cmd)
	assert.Error(t, err)
}

func TestCommentEventWorkerProxy_HandleForceApply_default(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	scope, _, err := metrics.NewLoggingScope(logger, "")
	assert.NoError(t, err)
	writer := &mockSnsWriter{}
	allocator := &testAllocator{
		t:                 t,
		expectedFeatureID: feature.PlatformMode,
		expectedFeatureCtx: feature.FeatureContext{
			RepoName: repoFullName,
		},
	}
	scheduler := &sync.SynchronousScheduler{Logger: logger}
	rootDeployer := &mockRootDeployer{}
	commentCreator := &mockCommentCreator{}
	statusUpdater := &mockStatusUpdater{
		expectedRepo:      testRepo,
		expectedPull:      testPull,
		expectedVCSStatus: models.QueuedVCSStatus,
		expectedCmd:       command.Apply.String(),
		expectedBody:      "Request received. Adding to the queue...",
		expectedT:         t,
	}
	getter := &testProjectCommandGetter{
		expectedProjectContext: []command.ProjectContext{
			{
				WorkflowModeType: valid.DefaultWorkflowMode,
				ProjectName:      "root1",
			},
			{
				WorkflowModeType: valid.DefaultWorkflowMode,
				ProjectName:      "root2",
			},
		},
	}

	cfg := valid.NewGlobalCfg("somedir")
	commentEventWorkerProxy := event.NewCommentEventWorkerProxy(logger, scope, writer, allocator, scheduler, rootDeployer, commentCreator, statusUpdater, cfg, getter)
	bufReq := buildRequest(t)
	commentEvent := event.Comment{
		Pull:     testPull,
		BaseRepo: testRepo,
	}
	cmd := &command.Comment{
		Name:       command.Apply,
		ForceApply: true,
	}
	err = commentEventWorkerProxy.Handle(context.Background(), bufReq, commentEvent, cmd)
	assert.NoError(t, err)
	assert.True(t, statusUpdater.isCalled)
	assert.False(t, commentCreator.isCalled)
	assert.False(t, rootDeployer.isCalled)
	assert.True(t, writer.isCalled)
}

func TestCommentEventWorkerProxy_HandleForceApply_BothModes(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	scope, _, err := metrics.NewLoggingScope(logger, "")
	assert.NoError(t, err)
	getter := &testProjectCommandGetter{
		expectedProjectContext: []command.ProjectContext{
			{
				WorkflowModeType: valid.PlatformWorkflowMode,
				ProjectName:      "root1",
			},
			{
				WorkflowModeType: valid.DefaultWorkflowMode,
				ProjectName:      "root2",
			},
		},
	}
	writer := &mockSnsWriter{}
	allocator := &testAllocator{
		t:                 t,
		expectedFeatureID: feature.PlatformMode,
		expectedFeatureCtx: feature.FeatureContext{
			RepoName: repoFullName,
		},
		expectedAllocation: true,
	}
	commentEvent := event.Comment{
		Pull:     testPull,
		BaseRepo: testRepo,
		HeadRepo: testRepo,
		User: models.User{
			Username: "someuser",
		},
		InstallationToken: 123,
	}
	scheduler := &sync.SynchronousScheduler{Logger: logger}
	rootDeployer := &testRootDeployer{
		expectedT: t,
		expectedOptions: deploy.RootDeployOptions{
			Repo:              testRepo,
			RootNames:         []string{"root1"},
			Branch:            testPull.HeadBranch,
			Revision:          testPull.HeadCommit,
			OptionalPullNum:   testPull.Num,
			Sender:            commentEvent.User,
			InstallationToken: commentEvent.InstallationToken,
			TriggerInfo: workflows.DeployTriggerInfo{
				Type:  workflows.ManualTrigger,
				Force: true,
			},
		},
	}
	commentCreator := &mockCommentCreator{}
	statusUpdater := &mockStatusUpdater{
		expectedRepo:      testRepo,
		expectedPull:      testPull,
		expectedVCSStatus: models.QueuedVCSStatus,
		expectedCmd:       command.Apply.String(),
		expectedBody:      "Request received. Adding to the queue...",
		expectedT:         t,
	}
	cfg := valid.NewGlobalCfg("somedir")
	commentEventWorkerProxy := event.NewCommentEventWorkerProxy(logger, scope, writer, allocator, scheduler, rootDeployer, commentCreator, statusUpdater, cfg, getter)
	bufReq := buildRequest(t)

	cmd := &command.Comment{
		Name:       command.Apply,
		ForceApply: true,
	}
	err = commentEventWorkerProxy.Handle(context.Background(), bufReq, commentEvent, cmd)
	assert.NoError(t, err)
	assert.False(t, commentCreator.isCalled)
	assert.True(t, rootDeployer.isCalled)
	assert.True(t, writer.isCalled)
	assert.True(t, statusUpdater.isCalled)
}

func TestCommentEventWorkerProxy_HandleForceApply_AllPlatform(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	scope, _, err := metrics.NewLoggingScope(logger, "")
	assert.NoError(t, err)
	getter := &testProjectCommandGetter{
		expectedProjectContext: []command.ProjectContext{
			{
				WorkflowModeType: valid.PlatformWorkflowMode,
				ProjectName:      "root1",
			},
			{
				WorkflowModeType: valid.PlatformWorkflowMode,
				ProjectName:      "root2",
			},
		},
	}
	writer := &mockSnsWriter{}
	allocator := &testAllocator{
		t:                 t,
		expectedFeatureID: feature.PlatformMode,
		expectedFeatureCtx: feature.FeatureContext{
			RepoName: repoFullName,
		},
		expectedAllocation: true,
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
	scheduler := &sync.SynchronousScheduler{Logger: logger}
	rootDeployer := &testRootDeployer{
		expectedT: t,
		expectedOptions: deploy.RootDeployOptions{
			Repo:              testRepo,
			RootNames:         []string{"root1", "root2"},
			Branch:            testPull.HeadBranch,
			Revision:          testPull.HeadCommit,
			OptionalPullNum:   testPull.Num,
			Sender:            commentEvent.User,
			InstallationToken: commentEvent.InstallationToken,
			TriggerInfo: workflows.DeployTriggerInfo{
				Type:  workflows.ManualTrigger,
				Force: true,
			},
		},
	}
	commentCreator := &mockCommentCreator{
		expectedT:       t,
		expectedRepo:    testRepo,
		expectedPull:    testPull.Num,
		expectedMessage: "⚠️ WARNING ⚠️\n\n You are force applying changes from your PR instead of merging into your default branch 🚀. This can have unpredictable consequences 🙏🏽 and should only be used in an emergency 🆘.\n\n To confirm behavior, review and confirm the plan within the generated atlantis/deploy GH check below.\n\n 𝐓𝐡𝐢𝐬 𝐚𝐜𝐭𝐢𝐨𝐧 𝐰𝐢𝐥𝐥 𝐛𝐞 𝐚𝐮𝐝𝐢𝐭𝐞𝐝.\n",
	}
	statusUpdater := &mockStatusUpdater{}
	cfg := valid.NewGlobalCfg("somedir")
	commentEventWorkerProxy := event.NewCommentEventWorkerProxy(logger, scope, writer, allocator, scheduler, rootDeployer, commentCreator, statusUpdater, cfg, getter)
	bufReq := buildRequest(t)
	cmd := &command.Comment{
		Name:       command.Apply,
		ForceApply: true,
	}
	err = commentEventWorkerProxy.Handle(context.Background(), bufReq, commentEvent, cmd)
	assert.NoError(t, err)
	assert.True(t, commentCreator.isCalled)
	assert.True(t, rootDeployer.isCalled)
	assert.False(t, writer.isCalled)
	assert.False(t, statusUpdater.isCalled)
}

func TestCommentEventWorkerProxy_HandleApplyComment_AllPlatformMode(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	scope, _, err := metrics.NewLoggingScope(logger, "")
	assert.NoError(t, err)
	getter := &testProjectCommandGetter{
		expectedProjectContext: []command.ProjectContext{
			{
				WorkflowModeType: valid.PlatformWorkflowMode,
				ProjectName:      "root1",
			},
			{
				WorkflowModeType: valid.PlatformWorkflowMode,
				ProjectName:      "root2",
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
	writer := &mockSnsWriter{}
	allocator := &testAllocator{
		t:                 t,
		expectedFeatureID: feature.PlatformMode,
		expectedFeatureCtx: feature.FeatureContext{
			RepoName: repoFullName,
		},
		expectedAllocation: true,
	}
	scheduler := &sync.SynchronousScheduler{Logger: logger}
	rootDeployer := &mockRootDeployer{}
	commentCreator := &mockCommentCreator{}
	statusUpdater := &mockStatusUpdater{}
	cfg := valid.NewGlobalCfg("somedir")
	commentEventWorkerProxy := event.NewCommentEventWorkerProxy(logger, scope, writer, allocator, scheduler, rootDeployer, commentCreator, statusUpdater, cfg, getter)
	bufReq := buildRequest(t)
	cmd := &command.Comment{
		Name: command.Apply,
	}
	err = commentEventWorkerProxy.Handle(context.Background(), bufReq, commentEvent, cmd)
	assert.NoError(t, err)
	assert.False(t, statusUpdater.isCalled)
	assert.False(t, commentCreator.isCalled)
	assert.False(t, rootDeployer.isCalled)
	assert.True(t, writer.isCalled)
}

func TestCommentEventWorkerProxy_HandleApplyComment_PartialMode(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	scope, _, err := metrics.NewLoggingScope(logger, "")
	assert.NoError(t, err)
	getter := &testProjectCommandGetter{
		expectedProjectContext: []command.ProjectContext{
			{
				WorkflowModeType: valid.PlatformWorkflowMode,
				ProjectName:      "root1",
			},
			{
				WorkflowModeType: valid.DefaultWorkflowMode,
				ProjectName:      "root2",
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
	writer := &mockSnsWriter{}
	allocator := &testAllocator{
		t:                 t,
		expectedFeatureID: feature.PlatformMode,
		expectedFeatureCtx: feature.FeatureContext{
			RepoName: repoFullName,
		},
		expectedAllocation: true,
	}
	scheduler := &sync.SynchronousScheduler{Logger: logger}
	rootDeployer := &mockRootDeployer{}
	commentCreator := &mockCommentCreator{}
	statusUpdater := &mockStatusUpdater{
		expectedRepo:      testRepo,
		expectedPull:      testPull,
		expectedVCSStatus: models.QueuedVCSStatus,
		expectedCmd:       command.Apply.String(),
		expectedBody:      "Request received. Adding to the queue...",
		expectedT:         t,
	}
	cfg := valid.NewGlobalCfg("somedir")
	commentEventWorkerProxy := event.NewCommentEventWorkerProxy(logger, scope, writer, allocator, scheduler, rootDeployer, commentCreator, statusUpdater, cfg, getter)
	bufReq := buildRequest(t)
	cmd := &command.Comment{
		Name: command.Apply,
	}
	err = commentEventWorkerProxy.Handle(context.Background(), bufReq, commentEvent, cmd)
	assert.NoError(t, err)
	assert.True(t, statusUpdater.isCalled)
	assert.False(t, commentCreator.isCalled)
	assert.False(t, rootDeployer.isCalled)
	assert.True(t, writer.isCalled)
}

func TestCommentEventWorkerProxy_HandlePlanComment_NoCmds(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	scope, _, err := metrics.NewLoggingScope(logger, "")
	assert.NoError(t, err)
	getter := &testProjectCommandGetter{}
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
	allocator := &testAllocator{
		t:                 t,
		expectedFeatureID: feature.PlatformMode,
		expectedFeatureCtx: feature.FeatureContext{
			RepoName: repoFullName,
		},
		expectedAllocation: true,
	}
	scheduler := &sync.SynchronousScheduler{Logger: logger}
	rootDeployer := &mockRootDeployer{}
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
	commentEventWorkerProxy := event.NewCommentEventWorkerProxy(logger, scope, writer, allocator, scheduler, rootDeployer, commentCreator, statusUpdater, cfg, getter)
	bufReq := buildRequest(t)
	cmd := &command.Comment{
		Name: command.Plan,
	}
	err = commentEventWorkerProxy.Handle(context.Background(), bufReq, commentEvent, cmd)
	assert.NoError(t, err)
	assert.True(t, statusUpdater.AllCalled())
	assert.False(t, commentCreator.isCalled)
	assert.False(t, rootDeployer.isCalled)
	assert.False(t, writer.isCalled)
}

func TestCommentEventWorkerProxy_HandleApplyComment_NoCmds(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	scope, _, err := metrics.NewLoggingScope(logger, "")
	assert.NoError(t, err)
	getter := &testProjectCommandGetter{}
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
	allocator := &testAllocator{
		t:                 t,
		expectedFeatureID: feature.PlatformMode,
		expectedFeatureCtx: feature.FeatureContext{
			RepoName: repoFullName,
		},
		expectedAllocation: true,
	}
	scheduler := &sync.SynchronousScheduler{Logger: logger}
	rootDeployer := &mockRootDeployer{}
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
	commentEventWorkerProxy := event.NewCommentEventWorkerProxy(logger, scope, writer, allocator, scheduler, rootDeployer, commentCreator, statusUpdater, cfg, getter)
	bufReq := buildRequest(t)
	cmd := &command.Comment{
		Name: command.Apply,
	}
	err = commentEventWorkerProxy.Handle(context.Background(), bufReq, commentEvent, cmd)
	assert.NoError(t, err)
	assert.True(t, statusUpdater.AllCalled())
	assert.False(t, commentCreator.isCalled)
	assert.False(t, rootDeployer.isCalled)
	assert.False(t, writer.isCalled)
}

func TestCommentEventWorkerProxy_HandlePlanComment_BothModes(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	scope, _, err := metrics.NewLoggingScope(logger, "")
	assert.NoError(t, err)
	getter := &testProjectCommandGetter{
		expectedProjectContext: []command.ProjectContext{
			{
				WorkflowModeType: valid.DefaultWorkflowMode,
				ProjectName:      "root1",
			},
			{
				WorkflowModeType: valid.PlatformWorkflowMode,
				ProjectName:      "root2",
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
	writer := &mockSnsWriter{}
	allocator := &testAllocator{
		t:                 t,
		expectedFeatureID: feature.PlatformMode,
		expectedFeatureCtx: feature.FeatureContext{
			RepoName: repoFullName,
		},
		expectedAllocation: true,
	}
	scheduler := &sync.SynchronousScheduler{Logger: logger}
	rootDeployer := &mockRootDeployer{}
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
	commentEventWorkerProxy := event.NewCommentEventWorkerProxy(logger, scope, writer, allocator, scheduler, rootDeployer, commentCreator, statusUpdater, cfg, getter)
	bufReq := buildRequest(t)
	cmd := &command.Comment{
		Name: command.Plan,
	}
	err = commentEventWorkerProxy.Handle(context.Background(), bufReq, commentEvent, cmd)
	assert.NoError(t, err)
	assert.True(t, statusUpdater.isCalled)
	assert.False(t, commentCreator.isCalled)
	assert.False(t, rootDeployer.isCalled)
	assert.True(t, writer.isCalled)
}

func TestCommentEventWorkerProxy_WriteError(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	scope, _, err := metrics.NewLoggingScope(logger, "")
	assert.NoError(t, err)
	getter := &testProjectCommandGetter{}
	writer := &mockSnsWriter{
		err: assert.AnError,
	}
	allocator := &testAllocator{
		t:                 t,
		expectedFeatureID: feature.PlatformMode,
		expectedFeatureCtx: feature.FeatureContext{
			RepoName: repoFullName,
		},
	}
	scheduler := &sync.SynchronousScheduler{Logger: logger}
	rootDeployer := &mockRootDeployer{}
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
	commentEventWorkerProxy := event.NewCommentEventWorkerProxy(logger, scope, writer, allocator, scheduler, rootDeployer, commentCreator, statusUpdater, cfg, getter)
	bufReq := buildRequest(t)
	commentEvent := event.Comment{
		Pull:     testPull,
		BaseRepo: testRepo,
	}
	cmd := &command.Comment{
		Name: command.Plan,
	}
	err = commentEventWorkerProxy.Handle(context.Background(), bufReq, commentEvent, cmd)
	assert.Error(t, err)
	assert.True(t, statusUpdater.isCalled)
	assert.False(t, commentCreator.isCalled)
	assert.False(t, rootDeployer.isCalled)
	assert.True(t, writer.isCalled)
}

func TestCommentEventWorkerProxy_HandleNoQueuedStatus(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	scope, _, err := metrics.NewLoggingScope(logger, "")
	assert.NoError(t, err)
	getter := &testProjectCommandGetter{
		expectedProjectContext: []command.ProjectContext{
			{
				WorkflowModeType: valid.DefaultWorkflowMode,
				ProjectName:      "root1",
			},
			{
				WorkflowModeType: valid.PlatformWorkflowMode,
				ProjectName:      "root2",
			},
		},
	}
	scheduler := &sync.SynchronousScheduler{Logger: logger}
	cfg := valid.NewGlobalCfg("somedir")
	// add branch regex
	cfg.Repos = []valid.Repo{
		{
			ID:          "/repo",
			BranchRegex: regexp.MustCompile("regex"),
		},
	}
	bufReq := buildRequest(t)
	allocator := &testAllocator{
		t:                 t,
		expectedFeatureID: feature.PlatformMode,
		expectedFeatureCtx: feature.FeatureContext{
			RepoName: repoFullName,
		},
	}

	forkedPull := models.PullRequest{
		BaseRepo: testRepo,
		HeadRepo: models.Repo{
			Owner: "new-owner",
		},
	}
	closedPull := models.PullRequest{
		BaseRepo: testRepo,
		State:    models.ClosedPullState,
	}
	cases := []struct {
		descriptor string
		allocator  *testAllocator
		command    *command.Comment
		event      event.Comment
	}{
		{
			descriptor: "non-plan/apply comment",
			allocator:  allocator,
			command:    &command.Comment{Name: command.Unlock},
			event: event.Comment{
				Pull:     testPull,
				BaseRepo: testRepo,
			},
		},
		{
			descriptor: "apply comment but platform mode enabled",
			allocator: &testAllocator{
				t:                 t,
				expectedFeatureID: feature.PlatformMode,
				expectedFeatureCtx: feature.FeatureContext{
					RepoName: repoFullName,
				},
				expectedAllocation: true,
			},
			command: &command.Comment{Name: command.Apply},
			event: event.Comment{
				Pull:     testPull,
				BaseRepo: testRepo,
			},
		},
		{
			descriptor: "forked PR",
			allocator:  allocator,
			command:    &command.Comment{Name: command.Plan},
			event: event.Comment{
				Pull:     forkedPull,
				BaseRepo: testRepo,
			},
		},
		{
			descriptor: "closed PR",
			allocator:  allocator,
			command:    &command.Comment{Name: command.Plan},
			event: event.Comment{
				Pull:     closedPull,
				BaseRepo: testRepo,
			},
		},
		{
			descriptor: "invalid base branch",
			allocator:  allocator,
			command:    &command.Comment{Name: command.Plan},
			event: event.Comment{
				Pull:     testPull,
				BaseRepo: testRepo,
			},
		},
	}
	for _, c := range cases {
		t.Run(c.descriptor, func(t *testing.T) {
			writer := &mockSnsWriter{}
			rootDeployer := &mockRootDeployer{}
			commentCreator := &mockCommentCreator{}
			statusUpdater := &mockStatusUpdater{
				expectedRepo:      testRepo,
				expectedPull:      testPull,
				expectedVCSStatus: models.QueuedVCSStatus,
				expectedCmd:       command.Plan.String(),
				expectedBody:      "Request received. Adding to the queue...",
				expectedT:         t,
			}
			commentEventWorkerProxy := event.NewCommentEventWorkerProxy(logger, scope, writer, c.allocator, scheduler, rootDeployer, commentCreator, statusUpdater, cfg, getter)
			err := commentEventWorkerProxy.Handle(context.Background(), bufReq, c.event, c.command)
			assert.NoError(t, err)
			assert.False(t, statusUpdater.isCalled)
			assert.False(t, commentCreator.isCalled)
			assert.False(t, rootDeployer.isCalled)
			assert.True(t, writer.isCalled)
		})
	}
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
