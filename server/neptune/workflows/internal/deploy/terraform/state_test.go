package terraform_test

import (
	"context"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github/markdown"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/notifier"
	internalTerraform "github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/version"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

type testActivities struct {
}

func (a *testActivities) GithubUpdateCheckRun(ctx context.Context, request activities.UpdateCheckRunRequest) (activities.UpdateCheckRunResponse, error) {
	return activities.UpdateCheckRunResponse{}, nil
}

func (a *testActivities) AuditJob(ctx context.Context, request activities.AuditJobRequest) error {
	return nil
}

type stateReceiveRequest struct {
	StatesToSend    []*state.Workflow
	DeploymentInfo  internalTerraform.DeploymentInfo
	ExpectedRequest notifier.GithubCheckRunRequest
	T               *testing.T
}

func testStateReceiveWorkflow(ctx workflow.Context, r stateReceiveRequest) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: 5 * time.Second,
	})
	ch := workflow.NewChannel(ctx)

	receiver := &internalTerraform.StateReceiver{
		Activity: &testActivities{},
		CheckRunSessionCache: &testCheckRunClient{
			expectedRequest: r.ExpectedRequest,
			expectedT:       r.T,
		},
	}

	workflow.Go(ctx, func(ctx workflow.Context) {
		for _, s := range r.StatesToSend {
			ch.Send(ctx, s)
		}
	})

	receiver.Receive(ctx, ch, r.DeploymentInfo)

	return nil
}

func TestStateReceive(t *testing.T) {
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
		Revision:   "12345",
	}

	cases := []struct {
		State                   *state.Workflow
		ExpectedCheckRunState   github.CheckRunState
		ExpectedAuditJobRequest *activities.AuditJobRequest
		ExpectedActions         []github.CheckRunAction
	}{
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.WaitingJobStatus,
				},
			},
			ExpectedCheckRunState: github.CheckRunPending,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.InProgressJobStatus,
				},
			},
			ExpectedCheckRunState: github.CheckRunPending,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.FailedJobStatus,
				},
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
			},
			ExpectedCheckRunState: github.CheckRunFailure,
		},
		{
			State: &state.Workflow{
				Plan: &state.Job{
					Output: jobOutput,
					Status: state.SuccessJobStatus,
				},
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
			},
			ExpectedCheckRunState: github.CheckRunPending,
			ExpectedAuditJobRequest: &activities.AuditJobRequest{
				Root:         internalDeploymentInfo.Root,
				Repo:         internalDeploymentInfo.Repo,
				Revision:     internalDeploymentInfo.Revision,
				State:        activities.AtlantisJobStateRunning,
				StartTime:    strconv.FormatInt(stTime.Unix(), 10),
				IsForceApply: false,
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
					Status:    state.FailedJobStatus,
					StartTime: stTime,
					EndTime:   endTime,
				},
			},
			ExpectedCheckRunState: github.CheckRunPending,
			ExpectedAuditJobRequest: &activities.AuditJobRequest{
				Root:         internalDeploymentInfo.Root,
				Repo:         internalDeploymentInfo.Repo,
				Revision:     internalDeploymentInfo.Revision,
				State:        activities.AtlantisJobStateFailure,
				StartTime:    strconv.FormatInt(stTime.Unix(), 10),
				EndTime:      strconv.FormatInt(endTime.Unix(), 10),
				IsForceApply: false,
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
					Status:    state.FailedJobStatus,
					StartTime: stTime,
					EndTime:   endTime,
				},
				Result: state.WorkflowResult{
					Status: state.CompleteWorkflowStatus,
					Reason: state.InternalServiceError,
				},
			},
			ExpectedCheckRunState: github.CheckRunFailure,
			ExpectedAuditJobRequest: &activities.AuditJobRequest{
				Root:         internalDeploymentInfo.Root,
				Repo:         internalDeploymentInfo.Repo,
				Revision:     internalDeploymentInfo.Revision,
				State:        activities.AtlantisJobStateFailure,
				StartTime:    strconv.FormatInt(stTime.Unix(), 10),
				EndTime:      strconv.FormatInt(endTime.Unix(), 10),
				IsForceApply: false,
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
					Status:    state.FailedJobStatus,
					StartTime: stTime,
					EndTime:   endTime,
				},
				Result: state.WorkflowResult{
					Status: state.CompleteWorkflowStatus,
					Reason: state.TimedOutError,
				},
			},
			ExpectedCheckRunState: github.CheckRunTimeout,
			ExpectedAuditJobRequest: &activities.AuditJobRequest{
				Root:         internalDeploymentInfo.Root,
				Repo:         internalDeploymentInfo.Repo,
				Revision:     internalDeploymentInfo.Revision,
				State:        activities.AtlantisJobStateFailure,
				StartTime:    strconv.FormatInt(stTime.Unix(), 10),
				EndTime:      strconv.FormatInt(endTime.Unix(), 10),
				IsForceApply: false,
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
					Status:    state.SuccessJobStatus,
					StartTime: stTime,
					EndTime:   endTime,
				},
				Result: state.WorkflowResult{
					Status: state.CompleteWorkflowStatus,
					Reason: state.SuccessfulCompletionReason,
				},
			},
			ExpectedCheckRunState: github.CheckRunSuccess,
			ExpectedAuditJobRequest: &activities.AuditJobRequest{
				Root:         internalDeploymentInfo.Root,
				Repo:         internalDeploymentInfo.Repo,
				Revision:     internalDeploymentInfo.Revision,
				State:        activities.AtlantisJobStateSuccess,
				StartTime:    strconv.FormatInt(stTime.Unix(), 10),
				EndTime:      strconv.FormatInt(endTime.Unix(), 10),
				IsForceApply: false,
			},
		},
	}

	for _, c := range cases {
		t.Run("old version", func(t *testing.T) {
			ts := testsuite.WorkflowTestSuite{}
			env := ts.NewTestWorkflowEnvironment()

			var a = &testActivities{}
			env.RegisterActivity(a)

			env.OnGetVersion(version.CacheCheckRunSessions, workflow.DefaultVersion, 1).Return(workflow.DefaultVersion)

			env.OnActivity(a.GithubUpdateCheckRun, mock.Anything, activities.UpdateCheckRunRequest{
				Title: "atlantis/deploy: root",
				State: c.ExpectedCheckRunState,
				Repo: github.Repo{
					Name: "hello",
				},
				Summary: markdown.RenderWorkflowStateTmpl(c.State),
				ID:      1,
				Actions: c.ExpectedActions,
			}).Return(activities.UpdateCheckRunResponse{}, nil)

			if c.ExpectedAuditJobRequest != nil {
				env.OnActivity(a.AuditJob, mock.Anything, *c.ExpectedAuditJobRequest).Return(nil)
			}

			env.ExecuteWorkflow(testStateReceiveWorkflow, stateReceiveRequest{
				StatesToSend:   []*state.Workflow{c.State},
				DeploymentInfo: internalDeploymentInfo,
			})

			env.AssertExpectations(t)

			err = env.GetWorkflowResult(nil)
			assert.NoError(t, err)

		})

		t.Run("new version", func(t *testing.T) {
			ts := testsuite.WorkflowTestSuite{}
			env := ts.NewTestWorkflowEnvironment()

			var a = &testActivities{}
			env.RegisterActivity(a)

			env.AssertNotCalled(t, "GithubUpdateCheckRun", activities.UpdateCheckRunRequest{
				Title: "atlantis/deploy: root",
				State: c.ExpectedCheckRunState,
				Repo: github.Repo{
					Name: "hello",
				},
				Summary: markdown.RenderWorkflowStateTmpl(c.State),
				ID:      1,
				Actions: c.ExpectedActions,
			})

			env.OnGetVersion(version.CacheCheckRunSessions, workflow.DefaultVersion, 1).Return(workflow.Version(1))

			if c.ExpectedAuditJobRequest != nil {
				env.OnActivity(a.AuditJob, mock.Anything, *c.ExpectedAuditJobRequest).Return(nil)
			}

			env.ExecuteWorkflow(testStateReceiveWorkflow, stateReceiveRequest{
				StatesToSend:   []*state.Workflow{c.State},
				DeploymentInfo: internalDeploymentInfo,
				ExpectedRequest: notifier.GithubCheckRunRequest{
					Title: "atlantis/deploy: root",
					Sha:   internalDeploymentInfo.Revision,
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
