package notifier_test

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/notifier"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github/markdown"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
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

type checkrunNotifierRequest struct {
	StatesToSend    []*state.Workflow
	NotifierInfo    notifier.Info
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
		if err := notifier.Notify(ctx, r.NotifierInfo, s); err != nil {
			return err
		}
	}

	return nil
}

func TestCheckRunNotifier(t *testing.T) {
	outputURL, err := url.Parse("www.nish.com")
	assert.NoError(t, err)

	deployMode := terraform.Deploy
	prMode := terraform.PR

	jobOutput := &state.JobOutput{
		URL: outputURL,
	}

	stTime := time.Now()
	endTime := stTime.Add(time.Second * 5)
	notifierInfo := notifier.Info{
		ID:   uuid.New(),
		Repo: github.Repo{Name: "hello"},
		Commit: github.Commit{
			Revision: "12345",
		},
		RootName: "root",
	}

	cases := []struct {
		State                 *state.Workflow
		ExpectedCheckRunState github.CheckRunState
		ExpectedActions       []github.CheckRunAction
	}{
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.WaitingJobStatus,
				},
				Mode: &deployMode,
			},
			ExpectedCheckRunState: github.CheckRunPending,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.InProgressJobStatus,
				},
				Mode: &deployMode,
			},
			ExpectedCheckRunState: github.CheckRunPending,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.FailedJobStatus,
				},
				Mode: &deployMode,
			},
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
				Mode: &deployMode,
			},
			ExpectedCheckRunState: github.CheckRunFailure,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.SuccessJobStatus,
				},
				Mode: &deployMode,
			},
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
				Mode: &deployMode,
			},
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
				Mode: &deployMode,
			},
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
				Mode: &deployMode,
			},
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
				Mode: &deployMode,
			},
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
				Mode: &deployMode,
			},
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
				Mode: &deployMode,
			},
			ExpectedCheckRunState: github.CheckRunSuccess,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.SuccessJobStatus,
				},
				Validate: &state.Job{
					Output: jobOutput,
					Status: state.WaitingJobStatus,
				},
				Mode: &prMode,
			},
			ExpectedCheckRunState: github.CheckRunQueued,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.SuccessJobStatus,
				},
				Validate: &state.Job{
					Output:    jobOutput,
					Status:    state.InProgressJobStatus,
					StartTime: stTime,
				},
				Mode: &prMode,
			},
			ExpectedCheckRunState: github.CheckRunPending,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.SuccessJobStatus,
				},
				Validate: &state.Job{
					Output:    jobOutput,
					Status:    state.FailedJobStatus,
					StartTime: stTime,
					EndTime:   endTime,
				},
				Mode: &prMode,
			},
			ExpectedCheckRunState: github.CheckRunPending,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.SuccessJobStatus,
				},
				Validate: &state.Job{
					Output:    jobOutput,
					Status:    state.FailedJobStatus,
					StartTime: stTime,
					EndTime:   endTime,
				},
				Result: state.WorkflowResult{
					Status: state.CompleteWorkflowStatus,
					Reason: state.InternalServiceError,
				},
				Mode: &prMode,
			},
			ExpectedCheckRunState: github.CheckRunFailure,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.SuccessJobStatus,
				},
				Validate: &state.Job{
					Output:    jobOutput,
					Status:    state.FailedJobStatus,
					StartTime: stTime,
					EndTime:   endTime,
				},
				Result: state.WorkflowResult{
					Status: state.CompleteWorkflowStatus,
					Reason: state.TimeoutError,
				},
				Mode: &prMode,
			},
			ExpectedCheckRunState: github.CheckRunTimeout,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.SuccessJobStatus,
				},
				Validate: &state.Job{
					Output:    jobOutput,
					Status:    state.SuccessJobStatus,
					StartTime: stTime,
					EndTime:   endTime,
				},
				Result: state.WorkflowResult{
					Status: state.CompleteWorkflowStatus,
					Reason: state.SuccessfulCompletionReason,
				},
				Mode: &prMode,
			},
			ExpectedCheckRunState: github.CheckRunSuccess,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.SuccessJobStatus,
				},
				Apply: &state.Job{
					Output: jobOutput,
					Status: state.RejectedJobStatus,
				},
				Result: state.WorkflowResult{
					Status: state.CompleteWorkflowStatus,
					Reason: state.SkippedCompletionReason,
				},
				Mode: &deployMode,
			},
			ExpectedCheckRunState: github.CheckRunSkipped,
		},
	}

	for _, c := range cases {
		t.Run("", func(t *testing.T) {
			ts := testsuite.WorkflowTestSuite{}
			env := ts.NewTestWorkflowEnvironment()

			var a = &testActivities{}
			env.RegisterActivity(a)

			env.ExecuteWorkflow(testCheckRunNotifier, checkrunNotifierRequest{
				StatesToSend: []*state.Workflow{c.State},
				NotifierInfo: notifierInfo,
				ExpectedRequest: notifier.GithubCheckRunRequest{
					Title: "atlantis/deploy: root",
					Sha:   notifierInfo.Commit.Revision,
					State: c.ExpectedCheckRunState,
					Repo: github.Repo{
						Name: "hello",
					},
					Summary: markdown.RenderWorkflowStateTmpl(c.State),
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
