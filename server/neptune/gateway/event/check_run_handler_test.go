package event_test

import (
	"context"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/neptune/sync"
	"testing"

	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event"
	"github.com/stretchr/testify/assert"
)

func TestCheckRunHandler(t *testing.T) {
	t.Run("unrelated check run", func(t *testing.T) {
		signaler := &testSignaler{}
		logger := logging.NewNoopCtxLogger(t)
		subject := event.CheckRunHandler{
			Logger:       logging.NewNoopCtxLogger(t),
			RootDeployer: &testRootDeployer{},

			// both are synchronous to keep our tests predictable
			SyncScheduler:  &sync.SynchronousScheduler{Logger: logger},
			AsyncScheduler: &sync.SynchronousScheduler{Logger: logger},
			DeploySignaler: &mockDeploySignaler{},
		}
		e := event.CheckRun{
			Name: "something",
		}
		err := subject.Handle(context.Background(), e)
		assert.NoError(t, err)
		assert.False(t, signaler.called)
	})

	t.Run("unsupported action", func(t *testing.T) {
		signaler := &mockDeploySignaler{}
		logger := logging.NewNoopCtxLogger(t)
		subject := event.CheckRunHandler{
			Logger:       logging.NewNoopCtxLogger(t),
			RootDeployer: &testRootDeployer{},
			// both are synchronous to keep our tests predictable
			SyncScheduler:  &sync.SynchronousScheduler{Logger: logger},
			AsyncScheduler: &sync.SynchronousScheduler{Logger: logger},
			DeploySignaler: signaler,
		}
		e := event.CheckRun{
			Action: event.WrappedCheckRunAction("test"),
			Name:   "atlantis/deploy: testroot",
		}
		err := subject.Handle(context.Background(), e)
		assert.NoError(t, err)
		assert.False(t, signaler.called)
	})

	t.Run("success", func(t *testing.T) {
		signaler := &mockDeploySignaler{}
		logger := logging.NewNoopCtxLogger(t)

		repo := models.Repo{DefaultBranch: "main"}
		branch := "something"
		user := models.User{
			Username: "nish",
		}
		sha := "12345"
		subject := event.CheckRunHandler{
			Logger: logging.NewNoopCtxLogger(t),
			RootDeployer: &testRootDeployer{
				expectedT: t,
				expectedOptions: event.RootDeployOptions{
					RootNames: []string{
						testRoot,
					},
					Repo:     repo,
					Branch:   branch,
					Sender:   user,
					Revision: sha,
				},
			},
			// both are synchronous to keep our tests predictable
			SyncScheduler:  &sync.SynchronousScheduler{Logger: logger},
			AsyncScheduler: &sync.SynchronousScheduler{Logger: logger},
			DeploySignaler: signaler,
		}
		e := event.CheckRun{
			Action:            event.WrappedCheckRunAction("test"),
			Name:              "atlantis/deploy: testroot",
			Repo:              repo,
			HeadSha:           sha,
			Branch:            branch,
			User:              user,
			InstallationToken: 2,
		}
		err := subject.Handle(context.Background(), e)
		assert.NoError(t, err)
	})

	t.Run("invalid rerequested branch", func(t *testing.T) {
		signaler := &mockDeploySignaler{}
		logger := logging.NewNoopCtxLogger(t)
		subject := event.CheckRunHandler{
			Logger: logging.NewNoopCtxLogger(t),
			RootDeployer: &testRootDeployer{
				expectedT: t,
			},
			// both are synchronous to keep our tests predictable
			SyncScheduler:  &sync.SynchronousScheduler{Logger: logger},
			AsyncScheduler: &sync.SynchronousScheduler{Logger: logger},
			DeploySignaler: signaler,
		}
		e := event.CheckRun{
			Action: event.WrappedCheckRunAction("test"),
			Name:   "atlantis/deploy: testroot",
			Repo:   models.Repo{DefaultBranch: "main"},
			Branch: "something",
		}
		err := subject.Handle(context.Background(), e)
		assert.NoError(t, err)
		assert.False(t, signaler.called)
	})

	t.Run("wrong requested actions object", func(t *testing.T) {
		signaler := &mockDeploySignaler{}
		logger := logging.NewNoopCtxLogger(t)
		subject := event.CheckRunHandler{
			Logger:       logging.NewNoopCtxLogger(t),
			RootDeployer: &testRootDeployer{},
			// both are synchronous to keep our tests predictable
			SyncScheduler:  &sync.SynchronousScheduler{Logger: logger},
			AsyncScheduler: &sync.SynchronousScheduler{Logger: logger},
			DeploySignaler: signaler,
		}
		e := event.CheckRun{
			Action: event.WrappedCheckRunAction("requested_action"),
			Name:   "atlantis/deploy: testroot",
		}
		err := subject.Handle(context.Background(), e)
		assert.Error(t, err)
		assert.False(t, signaler.called)
	})

	t.Run("unsupported action id", func(t *testing.T) {
		signaler := &mockDeploySignaler{}
		logger := logging.NewNoopCtxLogger(t)
		subject := event.CheckRunHandler{
			Logger:       logging.NewNoopCtxLogger(t),
			RootDeployer: &testRootDeployer{},
			// both are synchronous to keep our tests predictable
			SyncScheduler:  &sync.SynchronousScheduler{Logger: logger},
			AsyncScheduler: &sync.SynchronousScheduler{Logger: logger},
			DeploySignaler: signaler,
		}
		e := event.CheckRun{
			Action: event.RequestedActionChecksAction{
				Identifier: "some random thing",
			},
			Name: "atlantis/deploy: testroot",
		}
		err := subject.Handle(context.Background(), e)
		assert.Error(t, err)
		assert.False(t, signaler.called)
	})

	t.Run("plan signal success", func(t *testing.T) {
		signaler := &mockDeploySignaler{}
		user := models.User{Username: "nish"}
		workflowID := "wfid"
		logger := logging.NewNoopCtxLogger(t)
		subject := event.CheckRunHandler{
			Logger:       logging.NewNoopCtxLogger(t),
			RootDeployer: &testRootDeployer{},
			// both are synchronous to keep our tests predictable
			SyncScheduler:  &sync.SynchronousScheduler{Logger: logger},
			AsyncScheduler: &sync.SynchronousScheduler{Logger: logger},
			DeploySignaler: signaler,
		}
		e := event.CheckRun{
			Action: event.RequestedActionChecksAction{
				Identifier: "Confirm",
			},
			ExternalID: workflowID,
			User:       user,
			Name:       "atlantis/deploy: testroot",
		}
		err := subject.Handle(context.Background(), e)
		assert.NoError(t, err)
		assert.True(t, signaler.called)
	})

	t.Run("unlock signal success", func(t *testing.T) {
		user := models.User{Username: "nish"}
		workflowID := "testrepo||testroot"
		signaler := &mockDeploySignaler{}
		logger := logging.NewNoopCtxLogger(t)
		subject := event.CheckRunHandler{
			Logger:       logging.NewNoopCtxLogger(t),
			RootDeployer: &testRootDeployer{},
			// both are synchronous to keep our tests predictable
			SyncScheduler:  &sync.SynchronousScheduler{Logger: logger},
			AsyncScheduler: &sync.SynchronousScheduler{Logger: logger},
			DeploySignaler: signaler,
		}
		e := event.CheckRun{
			Action: event.RequestedActionChecksAction{
				Identifier: "Unlock",
			},
			ExternalID: workflowID,
			User:       user,
			Repo:       models.Repo{FullName: "testrepo"},
			Name:       "atlantis/deploy: testroot",
		}
		err := subject.Handle(context.Background(), e)
		assert.NoError(t, err)
		assert.True(t, signaler.called)
	})

	t.Run("non-deploy atlantis check run", func(t *testing.T) {
		user := models.User{Username: "nish"}
		workflowID := "testrepo||testroot"
		subject := event.CheckRunHandler{
			Logger: logging.NewNoopCtxLogger(t),
		}
		e := event.CheckRun{
			Action: event.RequestedActionChecksAction{
				Identifier: "Unlock",
			},
			ExternalID: workflowID,
			User:       user,
			Repo:       models.Repo{FullName: "testrepo"},
			Name:       "atlantis/plan: testroot",
		}
		err := subject.Handle(context.Background(), e)
		assert.NoError(t, err)
	})

	t.Run("signal error", func(t *testing.T) {
		signaler := &mockDeploySignaler{error: assert.AnError}
		user := models.User{Username: "nish"}
		workflowID := "wfid"
		logger := logging.NewNoopCtxLogger(t)
		subject := event.CheckRunHandler{
			Logger:       logging.NewNoopCtxLogger(t),
			RootDeployer: &testRootDeployer{},
			// both are synchronous to keep our tests predictable
			SyncScheduler:  &sync.SynchronousScheduler{Logger: logger},
			AsyncScheduler: &sync.SynchronousScheduler{Logger: logger},
			DeploySignaler: signaler,
		}
		e := event.CheckRun{
			Action: event.RequestedActionChecksAction{
				Identifier: "Confirm",
			},
			ExternalID: workflowID,
			User:       user,
			Name:       "atlantis/deploy: testroot",
		}
		err := subject.Handle(context.Background(), e)
		assert.Error(t, err)
		assert.True(t, signaler.called)
	})
}

type testRootDeployer struct {
	expectedT       *testing.T
	isCalled        bool
	expectedOptions event.RootDeployOptions
	error           error
}

func (m *testRootDeployer) Deploy(_ context.Context, options event.RootDeployOptions) error {
	assert.Equal(m.expectedT, m.expectedOptions, options)
	m.isCalled = true
	return m.error
}
