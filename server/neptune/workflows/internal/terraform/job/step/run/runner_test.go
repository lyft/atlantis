package run_test

// import (
// 	"context"
// 	"fmt"
// 	"path/filepath"
// 	"testing"

// 	"github.com/runatlantis/atlantis/server/neptune/workflows"
// 	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
// 	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
// 	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/job"
// 	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/steps"
// 	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/job/step/run"
// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/mock"
// 	"github.com/stretchr/testify/suite"
// 	"go.temporal.io/sdk/testsuite"
// )

// // func TestRunRunner(t *testing.T) {
// // 	/*
// // 		- Sets up env variables correctly
// // 		- Error when execute activity fails
// // 	*/
// // }

// type UnitTestSuite struct {
// 	suite.Suite
// 	testsuite.WorkflowTestSuite

// 	env *testsuite.TestActivityEnvironment
// }

// func (s *UnitTestSuite) SetupTest() {
// 	s.env = s.NewTestActivityEnvironment()
// }

// func (s *UnitTestSuite) TestRunRunner(t *testing.T) {
// 	runnner := run.Runner{}

// 	repoPath := filepath.Join("test", "repo")

// 	jobExectionCtx := &job.ExecutionContext{
// 		Path: "test-path",
// 	}
// 	rootInstance := &steps.RootInstance{
// 		Root: steps.Root{
// 			Name: "test-root",
// 			Path: filepath.Join(repoPath, "test-root"),
// 		},
// 		Repo: github.RepoInstance{
// 			Path:  repoPath,
// 			Name:  "test-repo",
// 			Owner: "test-owner",
// 			HeadCommit: github.Commit{
// 				Ref: "ref",
// 				Author: github.User{
// 					Username: "test-user",
// 				},
// 			},
// 		},
// 	}
// 	step := steps.Step{}

// 	s.env.RegisterActivity(workflows.TerraformActivities.ExecuteCommand, mock.Anything)

// 	s.env.ExecuteActivity(workflows.TerraformActivities.ExecuteCommand, mock.Anything, mock.Anything).Return(
// 		func(ctx context.Context, request activities.ExecuteCommandRequest) (activities.ExecuteCommandResponse, error) {
// 			// Assert the env variables are set properly
// 			fmt.Println(request.EnvVars)
// 			return activities.ExecuteCommandResponse{}, nil
// 		})

// 	out, err := runnner.Run(jobExectionCtx, rootInstance, step)

// 	assert.Nil(t, err)
// 	assert.Nil(t, out)
// }

// func TestRunRunner_ShouldSetupEnvVars(t *testing.T) {

// 	type suite struct {
// 		suite.Suite
// 		testsuite.WorkflowTestSuite
// 	}
// 	env := testsuite.TestActivityEnvironment

// 	runnner := run.Runner{}

// 	repoPath := filepath.Join("test", "repo")

// 	jobExectionCtx := &job.ExecutionContext{
// 		Path: "test-path",
// 	}
// 	rootInstance := &steps.RootInstance{
// 		Root: steps.Root{
// 			Name: "test-root",
// 			Path: filepath.Join(repoPath, "test-root"),
// 		},
// 		Repo: github.RepoInstance{
// 			Path:  repoPath,
// 			Name:  "test-repo",
// 			Owner: "test-owner",
// 			HeadCommit: github.Commit{
// 				Ref: "ref",
// 				Author: github.User{
// 					Username: "test-user",
// 				},
// 			},
// 		},
// 	}
// 	step := steps.Step{}

// 	s.env.OnActivity(workflows.TerraformActivities.ExecuteCommand, mock.Anything, mock.Anything).Return(
// 		func(ctx context.Context, request activities.ExecuteCommandRequest) (activities.ExecuteCommandResponse, error) {
// 			// Assert the env variables are set properly
// 			fmt.Println(request.EnvVars)
// 			return activities.ExecuteCommandResponse{}, nil
// 		})

// 	out, err := runnner.Run(jobExectionCtx, rootInstance, step)

// 	assert.Nil(t, err)
// 	assert.Nil(t, out)
// }
