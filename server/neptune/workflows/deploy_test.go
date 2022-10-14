package workflows_test

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-github/v45/github"
	"github.com/graymeta/stow"
	"github.com/graymeta/stow/local"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/core/runtime/cache"
	"github.com/runatlantis/atlantis/server/neptune/temporalworker/config"
	"github.com/runatlantis/atlantis/server/neptune/temporalworker/job"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	internalGithub "github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
)

type a struct {
	*workflows.GithubActivities
	*workflows.TerraformActivities
	*workflows.DeployActivities
}

func TestDeployWorkflow(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	dataDir := t.TempDir()

	err := os.Mkdir(filepath.Join(dataDir, "container"), os.ModePerm)
	assert.NoError(t, err)

	u, err := url.Parse("www.server.com")
	assert.NoError(t, err)

	cfg := config.Config{
		DeploymentConfig: valid.StoreConfig{
			BackendType: valid.LocalBackend,
			Config: stow.ConfigMap{
				local.ConfigKeyPath: dataDir,
			},
			ContainerName: "container",
			Prefix:        "prefix",
		},
		TerraformCfg: config.TerraformConfig{
			DefaultVersionStr: "1.0.2",
		},
		DataDir: dataDir,
		ServerCfg: config.ServerConfig{
			URL: u,
		},
		App: githubapp.Config{},
	}

	s := initAndRegisterActivities(t, env, cfg)

	env.RegisterWorkflow(workflows.Deploy)
	env.RegisterWorkflow(workflows.Terraform)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(workflows.DeployNewRevisionSignalID, workflows.DeployNewRevisionSignalRequest{
			Revision: "12345",
			Root: workflows.Root{
				Name: "my test root",
				Plan: workflows.Job{
					Steps: []workflows.Step{
						{
							StepName: "init",
						},
						{
							StepName: "plan",
						},
					},
				},
				Apply: workflows.Job{
					Steps: []workflows.Step{
						{
							StepName: "apply",
						},
					},
				},
				RepoRelPath: "terraform/mytestroot",
				PlanMode:    workflows.NormalPlanMode,
				Trigger:     workflows.MergeTrigger,
			},
		})
	}, 5*time.Second)

	env.RegisterDelayedCallback(func() {
		err := env.SignalWorkflowByID(s.githubClient.DeploymentID, workflows.TerraformPlanReviewSignalName, workflows.TerraformPlanReviewSignalRequest{
			Status: workflows.ApprovedPlanReviewStatus,
		})

		if err != nil {
			panic(err)
		}
	}, 30*time.Second)
	env.ExecuteWorkflow(workflows.Deploy, workflows.DeployRequest{})
	assert.NoError(t, env.GetWorkflowError())

	t.Log(s.githubClient.Updates)

	t.Fail()
}

type testSingletons struct {
	a            *a
	githubClient *testGithubClient
}

func initAndRegisterActivities(t *testing.T, env *testsuite.TestWorkflowEnvironment, cfg config.Config) *testSingletons {
	deployActivities, err := workflows.NewDeployActivities(cfg.DeploymentConfig)

	assert.NoError(t, err)

	terraformActivities, err := activities.NewTerraform(
		cfg.TerraformCfg,
		cfg.DataDir,
		cfg.ServerCfg.URL,
		&testStreamCloser{},
		activities.TerraformOptions{
			VersionCache: cache.NewLocalBinaryCache("terraform"),
		},
	)

	assert.NoError(t, err)

	githubClient := &testGithubClient{}

	githubActivities, err := activities.NewGithub(
		githubClient,
		cfg.DataDir,
		GetLocalTestRoot,
	)

	assert.NoError(t, err)

	env.RegisterActivity(terraformActivities)
	env.RegisterActivity(deployActivities)
	env.RegisterActivity(githubActivities)

	return &testSingletons{
		a: &a{
			GithubActivities: &workflows.GithubActivities{
				Github: githubActivities,
			},
			TerraformActivities: &workflows.TerraformActivities{
				Terraform: terraformActivities,
			},
			DeployActivities: deployActivities,
		},
		githubClient: githubClient,
	}

}

type testStreamCloser struct {
	*job.StreamHandler
}

func (sc *testStreamCloser) Stream(jobID string, msg string) {}

func (sc *testStreamCloser) CloseJob(ctx context.Context, jobID string) error {
	return nil
}

var fileContents = ` resource "null_resource" "null" {}
}`

func GetLocalTestRoot(dst, src string, ctx context.Context) error {
	err := os.Mkdir(dst, os.ModePerm)

	if err != nil {
		return errors.Wrapf(err, "creating directory at %s", dst)
	}

	if err := os.WriteFile(filepath.Join(dst, "main.tf"), []byte(fileContents), os.ModePerm); err != nil {
		return errors.Wrapf(err, "writing file")
	}

	return nil
}

type CheckRunUpdate struct {
	Summary    string
	Status     string
	Conclusion string
}

type testGithubClient struct {
	Updates      []CheckRunUpdate
	DeploymentID string
}

func (c *testGithubClient) CreateCheckRun(ctx internalGithub.Context, owner, repo string, opts github.CreateCheckRunOptions) (*github.CheckRun, *github.Response, error) {
	c.DeploymentID = opts.GetExternalID()
	return &github.CheckRun{
		ID: github.Int64(123),
	}, &github.Response{}, nil
}
func (c *testGithubClient) UpdateCheckRun(ctx internalGithub.Context, owner, repo string, checkRunID int64, opts github.UpdateCheckRunOptions) (*github.CheckRun, *github.Response, error) {
	c.DeploymentID = opts.GetExternalID()
	update := CheckRunUpdate{
		Summary:    opts.GetOutput().GetSummary(),
		Status:     opts.GetStatus(),
		Conclusion: opts.GetConclusion(),
	}

	c.Updates = append(c.Updates, update)

	return &github.CheckRun{}, &github.Response{}, nil
}
func (c *testGithubClient) GetArchiveLink(ctx internalGithub.Context, owner, repo string, archiveformat github.ArchiveFormat, opts *github.RepositoryContentGetOptions, followRedirects bool) (*url.URL, *github.Response, error) {
	url, _ := url.Parse("www.testurl.com")

	return url, &github.Response{}, nil
}
func (c *testGithubClient) CompareCommits(ctx internalGithub.Context, owner, repo string, base, head string, opts *github.ListOptions) (*github.CommitsComparison, *github.Response, error) {
	return &github.CommitsComparison{
		Status: github.String("ahead"),
	}, &github.Response{}, nil
}
