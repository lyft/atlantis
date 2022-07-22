package deploy_test

import (
	"testing"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/signals"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
)

func TestWorkflow_CompletesWithoutAnError(t *testing.T) {
	revision := "123456789"
	request := deploy.Request{
		GHRequestID: "some-id",
		Repository: deploy.Repo{
			FullName: "nish/repo",
			Owner:    "nish",
			Name:     "repo",
			URL:      "git@github.com:nish/repo.git",
		},
		Root: deploy.Root{
			Name: "root1",
		},
	}
	a := activities.NewDeploy()
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterActivity(a.FetchLatestDeployment)

	env.OnActivity(a.FetchLatestDeployment, activities.FetchLatestDeploymentRequest{
		RepositoryURL: request.Repository.URL,
		RootName:      request.Root.Name,
	}).Once()

	// Execute
	env.ExecuteWorkflow(deploy.Workflow, request)
	env.SignalWorkflow(signals.NewRevisionID, signals.NewRevisionRequest{
		Revision: revision,
	})

	// Validate
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())
}

func TestWorkflow_HandlesMultipleRevisions(t *testing.T) {
	revisions := []string{
		"123456789",
		"234567890",
	}
	request := deploy.Request{
		GHRequestID: "some-id",
		Repository: deploy.Repo{
			FullName: "nish/repo",
			Owner:    "nish",
			Name:     "repo",
			URL:      "git@github.com:nish/repo.git",
		},
		Root: deploy.Root{
			Name: "root1",
		},
	}

	a := activities.NewDeploy()
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterActivity(a.FetchLatestDeployment)

	env.OnActivity(a.FetchLatestDeployment, activities.FetchLatestDeploymentRequest{
		RepositoryURL: request.Repository.URL,
		RootName:      request.Root.Name,
	}).Times(len(revisions))

	// Execute
	env.ExecuteWorkflow(deploy.Workflow, request)

	for _, r := range revisions {
		env.SignalWorkflow(signals.NewRevisionID, signals.NewRevisionRequest{
			Revision: r,
		})
	}

	// Validate
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())
}
