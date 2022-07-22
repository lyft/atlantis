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
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	revision := "123456789"

	a := activities.NewDeploy()

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

	env.RegisterActivity(a.FetchLatestDeployment)

	env.OnActivity(a.FetchLatestDeployment, activities.FetchLatestDeploymentRequest{
		RepositoryURL: request.Repository.URL,
		RootName:      request.Root.Name,
	}).Once()

	env.ExecuteWorkflow(deploy.Workflow, request)

	env.SignalWorkflow(signals.NewRevisionID, signals.NewRevisionRequest{
		Revision: revision,
	})

	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())
}

func TestWorkflow_HandlesMultipleRevisions(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	revisions := []string{
		"123456789",
		"234567890",
	}

	a := activities.NewDeploy()

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

	env.RegisterActivity(a.FetchLatestDeployment)

	env.OnActivity(a.FetchLatestDeployment, activities.FetchLatestDeploymentRequest{
		RepositoryURL: request.Repository.URL,
		RootName:      request.Root.Name,
	}).Times(len(revisions))

	env.ExecuteWorkflow(deploy.Workflow, request)

	for _, r := range revisions {
		env.SignalWorkflow(signals.NewRevisionID, signals.NewRevisionRequest{
			Revision: r,
		})
	}

	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())
}
