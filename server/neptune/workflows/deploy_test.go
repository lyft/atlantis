package workflows_test

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/neptune/lyft/feature"

	"github.com/google/go-github/v45/github"
	"github.com/graymeta/stow"
	"github.com/graymeta/stow/local"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/config/raw"
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/legacy/core/runtime/cache"
	"github.com/runatlantis/atlantis/server/neptune/temporalworker/config"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
)

type a struct {
	*activities.Github
	*activities.Terraform
	*activities.Deploy
}

// we don't want to mess up all our gitconfig for testing purposes
type noopCredentialsRefresher struct{}

func (r noopCredentialsRefresher) Refresh(ctx context.Context, token int64) error {
	return nil
}

func TestDeployWorkflow(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	deployWorkflow := workflows.GetDeploy()

	env.RegisterWorkflow(deployWorkflow)
	env.RegisterWorkflow(workflows.Terraform)

	repo := workflows.Repo{
		FullName: "nish/repo",
		Owner:    "nish",
		Name:     "repo",
	}

	root := workflows.Root{
		Name: "mytestroot",
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
		RepoRelPath:  "terraform/mytestroot",
		PlanMode:     workflows.NormalPlanMode,
		Trigger:      workflows.MergeTrigger,
		TrackedFiles: raw.DefaultAutoPlanWhenModified,
	}

	revRequest := workflows.DeployNewRevisionSignalRequest{
		Revision: "12345",
		Root:     root,
		Repo:     repo,
	}

	s := initAndRegisterActivities(t, env)

	env.RegisterDelayedCallback(func() {
		signalWorkflow(env, revRequest)
	}, 5*time.Second)

	env.ExecuteWorkflow(deployWorkflow, workflows.DeployRequest{
		Root: workflows.DeployRequestRoot{
			Name: root.Name,
		},
		Repo: workflows.DeployRequestRepo{
			FullName: repo.FullName,
		},
	})
	assert.NoError(t, env.GetWorkflowError())

	// for now we just assert the correct number of updates were called.
	// asserting the output itself is a bit overkill tbh.

	// there should be 6 state changes that are reflected in our checks (3 state changes for plan and apply)
	assert.Len(t, s.githubClient.Updates, 7)

	// we should have output for 2 different jobs
	assert.Len(t, s.streamCloser.CapturedJobOutput, 2)
}

func signalWorkflow(env *testsuite.TestWorkflowEnvironment, revRequest workflows.DeployNewRevisionSignalRequest) {
	env.SignalWorkflow(workflows.DeployNewRevisionSignalID, revRequest)
}

type testSingletons struct {
	a            *a
	githubClient *testGithubClient
	streamCloser *testStreamCloser
}

func buildConfig(t *testing.T) config.Config {
	u, err := url.Parse("www.server.com")
	assert.NoError(t, err)

	dataDir := t.TempDir()

	// storage client uses this for it's local backend.
	err = os.Mkdir(filepath.Join(dataDir, "container"), os.ModePerm)
	assert.NoError(t, err)
	conftestVersion, err := version.NewVersion("0.25.0")
	assert.NoError(t, err)

	return config.Config{
		DeploymentConfig: valid.StoreConfig{
			BackendType: valid.LocalBackend,
			Config: stow.ConfigMap{
				local.ConfigKeyPath: dataDir,
			},
			ContainerName: "container",
			Prefix:        "prefix",
		},
		TemporalCfg: valid.Temporal{
			TerraformTaskQueue: "taskqueue",
		},
		TerraformCfg: config.TerraformConfig{
			DefaultVersion: "1.0.2",
		},
		ValidationConfig: config.ValidationConfig{
			DefaultVersion: conftestVersion,
		},
		DataDir: dataDir,
		ServerCfg: config.ServerConfig{
			URL: u,
		},
		App: githubapp.Config{},
	}
}

func initAndRegisterActivities(t *testing.T, env *testsuite.TestWorkflowEnvironment) *testSingletons {
	cfg := buildConfig(t)

	deployActivities, err := activities.NewDeploy(cfg.DeploymentConfig)

	assert.NoError(t, err)

	streamCloser := &testStreamCloser{
		CapturedJobOutput: make(map[string][]string),
	}

	terraformActivities, err := activities.NewTerraform(
		cfg.TerraformCfg,
		cfg.ValidationConfig,
		cfg.App,
		cfg.DataDir,
		cfg.ServerCfg.URL,
		cfg.TemporalCfg.TerraformTaskQueue,
		0,
		streamCloser,
		activities.TerraformOptions{
			TFVersionCache:          cache.NewLocalBinaryCache("terraform"),
			ConftestVersionCache:    cache.NewLocalBinaryCache("conftest"),
			GitCredentialsRefresher: noopCredentialsRefresher{},
		},
	)

	assert.NoError(t, err)

	githubClient := &testGithubClient{}
	githubActivities, err := activities.NewGithubWithClient(
		githubClient,
		cfg.DataDir,
		GetLocalTestRoot,
		&testAllocator{},
	)
	assert.NoError(t, err)

	env.RegisterActivity(terraformActivities)
	env.RegisterActivity(deployActivities)
	env.RegisterActivity(githubActivities)
	return &testSingletons{
		a: &a{
			Github:    githubActivities,
			Terraform: terraformActivities,
			Deploy:    deployActivities,
		},
		githubClient: githubClient,
		streamCloser: streamCloser,
	}
}

type testStreamCloser struct {
	CapturedJobOutput map[string][]string
}

func (sc *testStreamCloser) RegisterJob(id string) chan string {
	v := []string{}
	ch := make(chan string)
	go func() {
		for s := range ch {
			v = append(v, s)
		}
		sc.CapturedJobOutput[id] = v
	}()
	return ch
}

func (sc *testStreamCloser) CloseJob(ctx context.Context, jobID string) error {
	return nil
}

var fileContents = ` resource "null_resource" "null" {}
`

func GetLocalTestRoot(ctx context.Context, dst, src string) error {
	// dst will be the repo path here but we also need to create the root itself
	dst = filepath.Join(dst, "terraform", "mytestroot")
	err := os.MkdirAll(dst, os.ModePerm)

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

func (c *testGithubClient) CreateComment(ctx context.Context, owner string, repo string, number int, comment *github.IssueComment) (*github.IssueComment, *github.Response, error) {
	return &github.IssueComment{}, &github.Response{}, nil
}

func (c *testGithubClient) ListTeamMembers(ctx context.Context, org string, teamSlug string) ([]*github.User, error) {
	return []*github.User{}, nil
}

func (c *testGithubClient) ListCommits(ctx context.Context, owner string, repo string, number int) ([]*github.RepositoryCommit, error) {
	return []*github.RepositoryCommit{}, nil
}

func (c *testGithubClient) DismissReview(ctx context.Context, owner, repo string, number int, reviewID int64, review *github.PullRequestReviewDismissalRequest) (*github.PullRequestReview, *github.Response, error) {
	return &github.PullRequestReview{}, &github.Response{}, nil
}

func (c *testGithubClient) ListReviews(ctx context.Context, owner string, repo string, number int) ([]*github.PullRequestReview, error) {
	return []*github.PullRequestReview{
		{
			State: github.String("APPROVED"),
		},
	}, nil
}

func (c *testGithubClient) GetPullRequest(ctx context.Context, owner, repo string, number int) (*github.PullRequest, *github.Response, error) {
	return &github.PullRequest{}, &github.Response{}, nil
}

func (c *testGithubClient) CreateCheckRun(ctx context.Context, owner, repo string, opts github.CreateCheckRunOptions) (*github.CheckRun, *github.Response, error) {
	c.DeploymentID = opts.GetExternalID()
	return &github.CheckRun{
		ID: github.Int64(123),
	}, &github.Response{}, nil
}
func (c *testGithubClient) UpdateCheckRun(ctx context.Context, owner, repo string, checkRunID int64, opts github.UpdateCheckRunOptions) (*github.CheckRun, *github.Response, error) {
	c.DeploymentID = opts.GetExternalID()
	update := CheckRunUpdate{
		Summary:    opts.GetOutput().GetSummary(),
		Status:     opts.GetStatus(),
		Conclusion: opts.GetConclusion(),
	}

	c.Updates = append(c.Updates, update)

	return &github.CheckRun{}, &github.Response{}, nil
}
func (c *testGithubClient) GetArchiveLink(ctx context.Context, owner, repo string, archiveformat github.ArchiveFormat, opts *github.RepositoryContentGetOptions, followRedirects bool) (*url.URL, *github.Response, error) {
	url, _ := url.Parse("www.testurl.com")

	return url, &github.Response{Response: &http.Response{StatusCode: http.StatusFound}}, nil
}
func (c *testGithubClient) CompareCommits(ctx context.Context, owner, repo string, base, head string, opts *github.ListOptions) (*github.CommitsComparison, *github.Response, error) {
	return &github.CommitsComparison{
		Status: github.String("ahead"),
	}, &github.Response{}, nil
}

type testAllocator struct {
}

func (a *testAllocator) ShouldAllocate(featureID feature.Name, featureCtx feature.FeatureContext) (bool, error) {
	return false, nil
}
