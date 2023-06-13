package deploy_test

import (
	"context"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/neptune/gateway/config"
	"github.com/runatlantis/atlantis/server/neptune/gateway/deploy"
	"github.com/runatlantis/atlantis/server/vcs/provider/github"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/client"
)

const testRoot = "testroot"

func TestDeploy(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	version, err := version.NewVersion("1.0.3")
	assert.NoError(t, err)
	// use default values for testing
	deployOptions := deploy.RootDeployOptions{
		Repo: models.Repo{
			Name: "test",
		},
		Branch:          "some-branch",
		Revision:        "some-revision",
		OptionalPullNum: 1,
		RootNames:       []string{"some-root"},
		RepoFetcherOptions: &github.RepoFetcherOptions{
			CloneDepth: 2000,
		},
		InstallationToken: 2,
	}

	commit := &config.RepoCommit{
		Repo:          deployOptions.Repo,
		Branch:        deployOptions.Branch,
		Sha:           deployOptions.Revision,
		OptionalPRNum: deployOptions.OptionalPullNum,
	}

	t.Run("root config builder error", func(t *testing.T) {
		signaler := &mockDeploySignaler{}
		ctx := context.Background()
		deployer := deploy.RootDeployer{
			DeploySignaler: signaler,
			Logger:         logger,
			RootConfigBuilder: &mockRootConfigBuilder{
				expectedT:      t,
				expectedCommit: commit,
				expectedToken:  deployOptions.InstallationToken,
				expectedOptions: []config.BuilderOptions{
					{
						RootNames:          deployOptions.RootNames,
						RepoFetcherOptions: deployOptions.RepoFetcherOptions,
					},
				},
				error: assert.AnError,
			},
		}

		err := deployer.Deploy(ctx, deployOptions)
		assert.Error(t, err)
		assert.False(t, signaler.called)
	})

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		signaler := &mockDeploySignaler{run: testRun{}}
		rootCfg := valid.MergedProjectCfg{
			Name: testRoot,
			DeploymentWorkflow: valid.Workflow{
				Plan:  valid.DefaultPlanStage,
				Apply: valid.DefaultApplyStage,
			},
			TerraformVersion: version,
		}
		rootCfgs := []*valid.MergedProjectCfg{
			&rootCfg,
		}
		deployer := deploy.RootDeployer{
			DeploySignaler: signaler,
			Logger:         logger,
			RootConfigBuilder: &mockRootConfigBuilder{
				expectedT:      t,
				expectedCommit: commit,
				expectedToken:  deployOptions.InstallationToken,
				expectedOptions: []config.BuilderOptions{
					{
						RootNames:          deployOptions.RootNames,
						RepoFetcherOptions: deployOptions.RepoFetcherOptions,
					},
				},
				rootConfigs: rootCfgs,
			},
		}

		err := deployer.Deploy(ctx, deployOptions)
		assert.NoError(t, err)
		assert.True(t, signaler.called)
	})
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

type mockDeploySignaler struct {
	run    client.WorkflowRun
	error  error
	called bool
}

func (d *mockDeploySignaler) SignalWorkflow(_ context.Context, _ string, _ string, _ string, _ interface{}) error {
	d.called = true
	return d.error
}

func (d *mockDeploySignaler) SignalWithStartWorkflow(_ context.Context, _ *valid.MergedProjectCfg, _ deploy.RootDeployOptions) (client.WorkflowRun, error) {
	d.called = true
	return d.run, d.error
}
