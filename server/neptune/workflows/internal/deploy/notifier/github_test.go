package notifier_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github/markdown"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/notifier"
	internalTerraform "github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type testCheckRunClient struct {
	expectedRequest      notifier.GithubCheckRunRequest
	expectedDeploymentID string
	expectedT            *testing.T
}

func (t *testCheckRunClient) CreateOrUpdate(ctx workflow.Context, deploymentID string, request notifier.GithubCheckRunRequest) (int64, error) {
	assert.Equal(t.expectedT, t.expectedRequest, request)
	assert.Equal(t.expectedT, t.expectedDeploymentID, deploymentID)

	return 1, nil
}

func (a *testActivities) AuditJob(ctx context.Context, request activities.AuditJobRequest) error {
	return nil
}

type checkrunNotifierRequest struct {
	StatesToSend    []*state.Workflow
	DeploymentInfo  internalTerraform.DeploymentInfo
	ExpectedRequest notifier.GithubCheckRunRequest
	T               *testing.T
}

func testCheckRunNotifier(ctx workflow.Context, r checkrunNotifierRequest) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: 5 * time.Second,
	})

	notifier := &notifier.CheckRunNotifier{
		CheckRunSessionCache: &testCheckRunClient{
			expectedRequest: r.ExpectedRequest,
			expectedT:       r.T,
		},
	}

	for _, s := range r.StatesToSend {
		if err := notifier.Notify(ctx, r.DeploymentInfo, s); err != nil {
			return err
		}
	}

	return nil
}

func TestCheckRunNotifier(t *testing.T) {
	outputURL, err := url.Parse("www.nish.com")
	assert.NoError(t, err)

	jobOutput := &state.JobOutput{
		URL: outputURL,
	}

	stTime := time.Now()
	endTime := stTime.Add(time.Second * 5)
	internalDeploymentInfo := internalTerraform.DeploymentInfo{
		CheckRunID: 1,
		ID:         uuid.New(),
		Root:       terraform.Root{Name: "root"},
		Repo:       github.Repo{Name: "hello"},
		Commit: github.Commit{
			Revision: "12345",
		},
	}

	cases := []struct {
		State                 *state.Workflow
		Mode                  terraform.WorkflowMode
		ExpectedCheckRunState github.CheckRunState
		ExpectedActions       []github.CheckRunAction
	}{
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.WaitingJobStatus,
				},
			},
			Mode:                  terraform.Deploy,
			ExpectedCheckRunState: github.CheckRunPending,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.InProgressJobStatus,
				},
			},
			Mode:                  terraform.Deploy,
			ExpectedCheckRunState: github.CheckRunPending,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.FailedJobStatus,
				},
			},
			Mode:                  terraform.Deploy,
			ExpectedCheckRunState: github.CheckRunPending,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.FailedJobStatus,
				},
				Result: state.WorkflowResult{
					Status: state.CompleteWorkflowStatus,
					Reason: state.InternalServiceError,
				},
			},
			Mode:                  terraform.Deploy,
			ExpectedCheckRunState: github.CheckRunFailure,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.SuccessJobStatus,
				},
			},
			Mode:                  terraform.Deploy,
			ExpectedCheckRunState: github.CheckRunPending,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.SuccessJobStatus,
				},
				Apply: &state.Job{
					Output: jobOutput,
					Status: state.WaitingJobStatus,
					OnWaitingActions: state.JobActions{
						Actions: []state.JobAction{
							{
								ID:   "Confirm",
								Info: "confirm",
							},
						},
					},
				},
			},
			Mode:                  terraform.Deploy,
			ExpectedCheckRunState: github.CheckRunActionRequired,
			ExpectedActions: []github.CheckRunAction{
				{
					Label:       "Confirm",
					Description: "confirm",
				},
			},
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.SuccessJobStatus,
				},
				Apply: &state.Job{
					Output:    jobOutput,
					Status:    state.InProgressJobStatus,
					StartTime: stTime,
				},
			},
			Mode:                  terraform.Deploy,
			ExpectedCheckRunState: github.CheckRunPending,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.SuccessJobStatus,
				},
				Apply: &state.Job{
					Output:    jobOutput,
					Status:    state.FailedJobStatus,
					StartTime: stTime,
					EndTime:   endTime,
				},
			},
			Mode:                  terraform.Deploy,
			ExpectedCheckRunState: github.CheckRunPending,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.SuccessJobStatus,
				},
				Apply: &state.Job{
					Output:    jobOutput,
					Status:    state.FailedJobStatus,
					StartTime: stTime,
					EndTime:   endTime,
				},
				Result: state.WorkflowResult{
					Status: state.CompleteWorkflowStatus,
					Reason: state.InternalServiceError,
				},
			},
			Mode:                  terraform.Deploy,
			ExpectedCheckRunState: github.CheckRunFailure,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.SuccessJobStatus,
				},
				Apply: &state.Job{
					Output:    jobOutput,
					Status:    state.FailedJobStatus,
					StartTime: stTime,
					EndTime:   endTime,
				},
				Result: state.WorkflowResult{
					Status: state.CompleteWorkflowStatus,
					Reason: state.TimeoutError,
				},
			},
			Mode:                  terraform.Deploy,
			ExpectedCheckRunState: github.CheckRunTimeout,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.SuccessJobStatus,
				},
				Apply: &state.Job{
					Output:    jobOutput,
					Status:    state.SuccessJobStatus,
					StartTime: stTime,
					EndTime:   endTime,
				},
				Result: state.WorkflowResult{
					Status: state.CompleteWorkflowStatus,
					Reason: state.SuccessfulCompletionReason,
				},
			},
			Mode:                  terraform.Deploy,
			ExpectedCheckRunState: github.CheckRunSuccess,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.SuccessJobStatus,
				},
				PolicyCheck: &state.Job{
					Output: jobOutput,
					Status: state.WaitingJobStatus,
				},
			},
			Mode:                  terraform.PR,
			ExpectedCheckRunState: github.CheckRunQueued,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.SuccessJobStatus,
				},
				PolicyCheck: &state.Job{
					Output:    jobOutput,
					Status:    state.InProgressJobStatus,
					StartTime: stTime,
				},
			},
			Mode:                  terraform.PR,
			ExpectedCheckRunState: github.CheckRunPending,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.SuccessJobStatus,
				},
				PolicyCheck: &state.Job{
					Output:    jobOutput,
					Status:    state.FailedJobStatus,
					StartTime: stTime,
					EndTime:   endTime,
				},
			},
			Mode:                  terraform.PR,
			ExpectedCheckRunState: github.CheckRunPending,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.SuccessJobStatus,
				},
				PolicyCheck: &state.Job{
					Output:    jobOutput,
					Status:    state.FailedJobStatus,
					StartTime: stTime,
					EndTime:   endTime,
				},
				Result: state.WorkflowResult{
					Status: state.CompleteWorkflowStatus,
					Reason: state.InternalServiceError,
				},
			},
			Mode:                  terraform.PR,
			ExpectedCheckRunState: github.CheckRunFailure,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.SuccessJobStatus,
				},
				PolicyCheck: &state.Job{
					Output:    jobOutput,
					Status:    state.FailedJobStatus,
					StartTime: stTime,
					EndTime:   endTime,
				},
				Result: state.WorkflowResult{
					Status: state.CompleteWorkflowStatus,
					Reason: state.TimeoutError,
				},
			},
			Mode:                  terraform.PR,
			ExpectedCheckRunState: github.CheckRunTimeout,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.SuccessJobStatus,
				},
				PolicyCheck: &state.Job{
					Output:    jobOutput,
					Status:    state.SuccessJobStatus,
					StartTime: stTime,
					EndTime:   endTime,
				},
				Result: state.WorkflowResult{
					Status: state.CompleteWorkflowStatus,
					Reason: state.SuccessfulCompletionReason,
				},
			},
			Mode:                  terraform.PR,
			ExpectedCheckRunState: github.CheckRunSuccess,
		},
	}

	for _, c := range cases {
		t.Run("", func(t *testing.T) {
			ts := testsuite.WorkflowTestSuite{}
			env := ts.NewTestWorkflowEnvironment()

			var a = &testActivities{}
			env.RegisterActivity(a)

			env.ExecuteWorkflow(testCheckRunNotifier, checkrunNotifierRequest{
				StatesToSend:   []*state.Workflow{c.State},
				DeploymentInfo: internalDeploymentInfo,
				ExpectedRequest: notifier.GithubCheckRunRequest{
					Title: "atlantis/deploy: root",
					Sha:   internalDeploymentInfo.Commit.Revision,
					State: c.ExpectedCheckRunState,
					Repo: github.Repo{
						Name: "hello",
					},
					Summary: markdown.RenderWorkflowStateTmpl(c.State, c.Mode),
					Actions: c.ExpectedActions,
				},
				T: t,
			})

			env.AssertExpectations(t)

			err = env.GetWorkflowResult(nil)
			assert.NoError(t, err)
		})
	}
}
