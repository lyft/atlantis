package event_test

import (
	"context"
	"github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event"
	"github.com/runatlantis/atlantis/server/neptune/gateway/sync"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckSuiteHandler(t *testing.T) {
	branch := "branch"
	version, err := version.NewVersion("1.0.3")
	assert.NoError(t, err)
	t.Run("success", func(t *testing.T) {
		rootCfg := valid.MergedProjectCfg{
			Name: testRoot,
			DeploymentWorkflow: valid.Workflow{
				Plan:  valid.DefaultPlanStage,
				Apply: valid.DefaultApplyStage,
			},
			TerraformVersion: version,
			WorkflowMode:     valid.PlatformWorkflowMode,
		}
		rootCfgs := []*valid.MergedProjectCfg{
			&rootCfg,
		}
		rootConfigBuilder := &mockRootConfigBuilder{
			rootConfigs: rootCfgs,
		}
		signaler := &mockDeploySignaler{}
		logger := logging.NewNoopCtxLogger(t)
		subject := event.CheckSuiteHandler{
			Logger:            logging.NewNoopCtxLogger(t),
			RootConfigBuilder: rootConfigBuilder,
			Scheduler:         &sync.SynchronousScheduler{Logger: logger},
			DeploySignaler:    signaler,
		}
		e := event.CheckSuite{
			Action: event.WrappedCheckRunAction(event.ReRequestedActionType),
			Branch: branch,
			Repo:   models.Repo{DefaultBranch: branch},
		}
		err := subject.Handle(context.Background(), e)
		assert.NoError(t, err)
		assert.True(t, signaler.called)
	})
	t.Run("unsupported action", func(t *testing.T) {
		signaler := &mockDeploySignaler{}
		logger := logging.NewNoopCtxLogger(t)
		subject := event.CheckSuiteHandler{
			Logger:            logging.NewNoopCtxLogger(t),
			RootConfigBuilder: &mockRootConfigBuilder{},
			Scheduler:         &sync.SynchronousScheduler{Logger: logger},
			DeploySignaler:    signaler,
		}
		e := event.CheckSuite{
			Action: event.WrappedCheckRunAction("something"),
		}
		err := subject.Handle(context.Background(), e)
		assert.NoError(t, err)
		assert.False(t, signaler.called)
	})

	t.Run("invalid branch", func(t *testing.T) {
		signaler := &mockDeploySignaler{}
		logger := logging.NewNoopCtxLogger(t)
		subject := event.CheckSuiteHandler{
			Logger:            logging.NewNoopCtxLogger(t),
			RootConfigBuilder: &mockRootConfigBuilder{},
			Scheduler:         &sync.SynchronousScheduler{Logger: logger},
			DeploySignaler:    signaler,
		}
		e := event.CheckSuite{
			Action: event.WrappedCheckRunAction(event.ReRequestedActionType),
			Branch: "something",
			Repo:   models.Repo{DefaultBranch: branch},
		}
		err := subject.Handle(context.Background(), e)
		assert.NoError(t, err)
		assert.False(t, signaler.called)
	})
	t.Run("failed root builder", func(t *testing.T) {
		signaler := &mockDeploySignaler{}
		logger := logging.NewNoopCtxLogger(t)
		subject := event.CheckSuiteHandler{
			Logger:            logging.NewNoopCtxLogger(t),
			RootConfigBuilder: &mockRootConfigBuilder{error: assert.AnError},
			Scheduler:         &sync.SynchronousScheduler{Logger: logger},
			DeploySignaler:    signaler,
		}
		e := event.CheckSuite{
			Action: event.WrappedCheckRunAction(event.ReRequestedActionType),
			Branch: branch,
			Repo:   models.Repo{DefaultBranch: branch},
		}
		err := subject.Handle(context.Background(), e)
		assert.Error(t, err)
		assert.False(t, signaler.called)
	})
	t.Run("failed deploy signaler", func(t *testing.T) {
		rootCfg := valid.MergedProjectCfg{
			Name: testRoot,
			DeploymentWorkflow: valid.Workflow{
				Plan:  valid.DefaultPlanStage,
				Apply: valid.DefaultApplyStage,
			},
			TerraformVersion: version,
			WorkflowMode:     valid.PlatformWorkflowMode,
		}
		rootCfgs := []*valid.MergedProjectCfg{
			&rootCfg,
		}
		rootConfigBuilder := &mockRootConfigBuilder{
			rootConfigs: rootCfgs,
		}
		signaler := &mockDeploySignaler{error: assert.AnError}
		logger := logging.NewNoopCtxLogger(t)
		subject := event.CheckSuiteHandler{
			Logger:            logging.NewNoopCtxLogger(t),
			RootConfigBuilder: rootConfigBuilder,
			Scheduler:         &sync.SynchronousScheduler{Logger: logger},
			DeploySignaler:    signaler,
		}
		e := event.CheckSuite{
			Action: event.WrappedCheckRunAction(event.ReRequestedActionType),
			Branch: branch,
			Repo:   models.Repo{DefaultBranch: branch},
		}
		err := subject.Handle(context.Background(), e)
		assert.Error(t, err)
		assert.True(t, signaler.called)
	})
}
