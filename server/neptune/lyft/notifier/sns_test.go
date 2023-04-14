package notifier_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/runatlantis/atlantis/server/neptune/lyft/activities"
	"github.com/runatlantis/atlantis/server/neptune/lyft/notifier"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/plugins"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type testSNSActivities struct {
	ExpectedT       *testing.T
	ExpectedRequest activities.AuditJobRequest
	Called          bool
}

func (a *testSNSActivities) AuditJob(ctx context.Context, request activities.AuditJobRequest) error {
	assert.Equal(a.ExpectedT, a.ExpectedRequest, request)

	a.Called = true

	return nil
}

type snsNotifierRequest struct {
	StatesToSend   []*plugins.TerraformWorkflowState
	DeploymentInfo plugins.TerraformDeploymentInfo
	T              *testing.T
}

func testSNSNotifierWorkflow(ctx workflow.Context, r snsNotifierRequest) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: 5 * time.Second,
	})

	var a *testSNSActivities
	notifier := &notifier.SNSNotifier{
		Activity: a,
	}

	for _, s := range r.StatesToSend {
		if err := notifier.Notify(ctx, r.DeploymentInfo, s); err != nil {
			return err
		}
	}

	return nil
}

func TestSNSNotifier_SendsMessage(t *testing.T) {
	stTime := time.Now()
	endTime := stTime.Add(time.Second * 5)
	internalDeploymentInfo := plugins.TerraformDeploymentInfo{
		ID:         uuid.New(),
		Root:       terraform.Root{Name: "root"},
		Repo:       github.Repo{Name: "hello"},
		Commit: github.Commit{
			Revision: "12345",
		},
	}

	cases := []struct {
		State                   *plugins.TerraformWorkflowState
		ExpectedAuditJobRequest activities.AuditJobRequest
	}{
		{
			State: &plugins.TerraformWorkflowState{
				Plan: &plugins.JobState{
					Status: plugins.SuccessJobStatus,
				},
				Apply: &plugins.JobState{
					Status:    plugins.InProgressJobStatus,
					StartTime: stTime,
				},
			},
			ExpectedAuditJobRequest: activities.AuditJobRequest{
				Root:         internalDeploymentInfo.Root,
				Repo:         internalDeploymentInfo.Repo,
				Revision:     internalDeploymentInfo.Commit.Revision,
				State:        activities.AtlantisJobStateRunning,
				StartTime:    strconv.FormatInt(stTime.Unix(), 10),
				IsForceApply: false,
			},
		},
		{
			State: &plugins.TerraformWorkflowState{
				Plan: &plugins.JobState{
					Status: plugins.SuccessJobStatus,
				},
				Apply: &plugins.JobState{
					Status:    plugins.FailedJobStatus,
					StartTime: stTime,
					EndTime:   endTime,
				},
			},
			ExpectedAuditJobRequest: activities.AuditJobRequest{
				Root:         internalDeploymentInfo.Root,
				Repo:         internalDeploymentInfo.Repo,
				Revision:     internalDeploymentInfo.Commit.Revision,
				State:        activities.AtlantisJobStateFailure,
				StartTime:    strconv.FormatInt(stTime.Unix(), 10),
				EndTime:      strconv.FormatInt(endTime.Unix(), 10),
				IsForceApply: false,
			},
		},
		{
			State: &plugins.TerraformWorkflowState{
				Plan: &plugins.JobState{
					Status: plugins.SuccessJobStatus,
				},
				Apply: &plugins.JobState{
					Status:    plugins.FailedJobStatus,
					StartTime: stTime,
					EndTime:   endTime,
				},
			},
			ExpectedAuditJobRequest: activities.AuditJobRequest{
				Root:         internalDeploymentInfo.Root,
				Repo:         internalDeploymentInfo.Repo,
				Revision:     internalDeploymentInfo.Commit.Revision,
				State:        activities.AtlantisJobStateFailure,
				StartTime:    strconv.FormatInt(stTime.Unix(), 10),
				EndTime:      strconv.FormatInt(endTime.Unix(), 10),
				IsForceApply: false,
			},
		},
		{
			State: &plugins.TerraformWorkflowState{
				Plan: &plugins.JobState{
					Status: plugins.SuccessJobStatus,
				},
				Apply: &plugins.JobState{
					Status:    plugins.FailedJobStatus,
					StartTime: stTime,
					EndTime:   endTime,
				},
			},
			ExpectedAuditJobRequest: activities.AuditJobRequest{
				Root:         internalDeploymentInfo.Root,
				Repo:         internalDeploymentInfo.Repo,
				Revision:     internalDeploymentInfo.Commit.Revision,
				State:        activities.AtlantisJobStateFailure,
				StartTime:    strconv.FormatInt(stTime.Unix(), 10),
				EndTime:      strconv.FormatInt(endTime.Unix(), 10),
				IsForceApply: false,
			},
		},
		{
			State: &plugins.TerraformWorkflowState{
				Plan: &plugins.JobState{
					Status: plugins.SuccessJobStatus,
				},
				Apply: &plugins.JobState{
					Status:    plugins.SuccessJobStatus,
					StartTime: stTime,
					EndTime:   endTime,
				},
			},
			ExpectedAuditJobRequest: activities.AuditJobRequest{
				Root:         internalDeploymentInfo.Root,
				Repo:         internalDeploymentInfo.Repo,
				Revision:     internalDeploymentInfo.Commit.Revision,
				State:        activities.AtlantisJobStateSuccess,
				StartTime:    strconv.FormatInt(stTime.Unix(), 10),
				EndTime:      strconv.FormatInt(endTime.Unix(), 10),
				IsForceApply: false,
			},
		},
	}

	for _, c := range cases {
		t.Run("", func(t *testing.T) {
			ts := testsuite.WorkflowTestSuite{}
			env := ts.NewTestWorkflowEnvironment()

			var a = &testSNSActivities{
				ExpectedT:       t,
				ExpectedRequest: c.ExpectedAuditJobRequest,
			}
			env.RegisterActivity(a)

			env.ExecuteWorkflow(testSNSNotifierWorkflow, snsNotifierRequest{
				StatesToSend:   []*plugins.TerraformWorkflowState{c.State},
				DeploymentInfo: internalDeploymentInfo,
				T:              t,
			})

			err := env.GetWorkflowResult(nil)
			assert.NoError(t, err)
			assert.True(t, a.Called)
		})
	}
}

func TestSNSNotifier_IfApplyJobNil(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	internalDeploymentInfo := plugins.TerraformDeploymentInfo{
		ID:         uuid.New(),
		Root:       terraform.Root{Name: "root"},
		Repo:       github.Repo{Name: "hello"},
		Commit: github.Commit{
			Revision: "12345",
		},
	}

	var a = &testSNSActivities{}
	env.RegisterActivity(a)

	env.ExecuteWorkflow(testSNSNotifierWorkflow, snsNotifierRequest{
		StatesToSend: []*plugins.TerraformWorkflowState{
			{
				Plan: &plugins.JobState{
					Status: plugins.SuccessJobStatus,
				},
			},
		},
		DeploymentInfo: internalDeploymentInfo,
		T:              t,
	})

	err := env.GetWorkflowResult(nil)
	assert.NoError(t, err)
	assert.False(t, a.Called)
}
