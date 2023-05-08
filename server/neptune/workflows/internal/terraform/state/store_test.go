package state_test

import (
	"bytes"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"github.com/stretchr/testify/assert"
)

const workflowID = "id"

type testNotifier struct {
	expectedState *state.Workflow
	t             *testing.T
	called        bool
}

func (n *testNotifier) notify(state *state.Workflow) error {
	n.called = true
	assert.Equal(n.t, n.expectedState, state)
	return nil
}

func TestUpdateApprovalActions(t *testing.T) {
	route := &mux.Route{}
	route.Path("/jobs/{job-id}")

	exoectedURL, err := url.Parse("www.test.com/jobs/1234")
	assert.NoError(t, err)
	deployMode := terraform.Deploy

	jobID := bytes.NewBufferString("1234")
	notifier := &testNotifier{
		expectedState: &state.Workflow{
			Mode: &deployMode,
			Apply: &state.Job{
				Status: state.WaitingJobStatus,
				Output: &state.JobOutput{
					URL: exoectedURL,
				},
				ID: jobID.String(),
			},
			ID: workflowID,
		},
		t: t,
	}

	subject := state.NewWorkflowStore(notifier.notify, terraform.Deploy, workflowID)

	baseURL := bytes.NewBufferString("www.test.com")

	// init and then update actions
	err = subject.InitApplyJob(jobID, baseURL)
	assert.NoError(t, err)

	notifier.expectedState.Apply.OnWaitingActions = state.JobActions{
		Actions: []state.JobAction{
			{
				ID:   state.ConfirmAction,
				Info: "Confirm this plan to proceed to apply",
			},
			{
				ID:   state.RejectAction,
				Info: "Reject this plan to prevent the apply",
			},
		},
		Summary: "some reason",
	}

	err = subject.UpdateApprovalActions(terraform.PlanApproval{
		Type:   terraform.ManualApproval,
		Reason: "some reason",
	})

	assert.NoError(t, err)
}

func TestInitPlanJob(t *testing.T) {
	exoectedURL, err := url.Parse("www.test.com/jobs/1234")
	assert.NoError(t, err)
	deployMode := terraform.Deploy

	jobID := bytes.NewBufferString("1234")
	notifier := &testNotifier{
		expectedState: &state.Workflow{
			Mode: &deployMode,
			Plan: &state.Job{
				Status: state.WaitingJobStatus,
				Output: &state.JobOutput{
					URL: exoectedURL,
				},
				ID: jobID.String(),
			},
			ID: workflowID,
		},
		t: t,
	}

	subject := state.NewWorkflowStore(notifier.notify, terraform.Deploy, workflowID)

	baseURL := bytes.NewBufferString("www.test.com")

	err = subject.InitPlanJob(jobID, baseURL)
	assert.NoError(t, err)
	assert.True(t, notifier.called)
}

func TestInitApplyJob(t *testing.T) {
	route := &mux.Route{}
	route.Path("/jobs/{job-id}")

	exoectedURL, err := url.Parse("www.test.com/jobs/1234")
	assert.NoError(t, err)
	deployMode := terraform.Deploy

	jobID := bytes.NewBufferString("1234")
	notifier := &testNotifier{
		expectedState: &state.Workflow{
			Mode: &deployMode,
			Apply: &state.Job{
				Status: state.WaitingJobStatus,
				Output: &state.JobOutput{
					URL: exoectedURL,
				},
				ID: jobID.String(),
			},
			ID: workflowID,
		},
		t: t,
	}

	subject := state.NewWorkflowStore(notifier.notify, terraform.Deploy, workflowID)

	baseURL := bytes.NewBufferString("www.test.com")

	err = subject.InitApplyJob(jobID, baseURL)
	assert.NoError(t, err)
	assert.True(t, notifier.called)
}

func TestUpdateApplyJob(t *testing.T) {
	route := &mux.Route{}
	route.Path("/jobs/{job-id}")

	stTime := time.Now()
	endTime := stTime.Add(time.Second * 10)
	exoectedURL, err := url.Parse("www.test.com/jobs/1234")
	assert.NoError(t, err)
	deployMode := terraform.Deploy

	jobID := bytes.NewBufferString("1234")
	notifier := &testNotifier{
		expectedState: &state.Workflow{
			Mode: &deployMode,
			Apply: &state.Job{
				Status: state.WaitingJobStatus,
				Output: &state.JobOutput{
					URL: exoectedURL,
				},
				ID: jobID.String(),
			},
			ID: workflowID,
		},
		t: t,
	}

	subject := state.NewWorkflowStore(notifier.notify, terraform.Deploy, workflowID)

	baseURL := bytes.NewBufferString("www.test.com")

	// init and then update
	err = subject.InitApplyJob(jobID, baseURL)
	assert.NoError(t, err)

	notifier.expectedState.Apply.Status = state.InProgressJobStatus
	notifier.expectedState.Apply.StartTime = stTime

	err = subject.UpdateApplyJobWithStatus(state.InProgressJobStatus, state.UpdateOptions{
		StartTime: stTime,
	})
	assert.NoError(t, err)

	notifier.expectedState.Apply.Status = state.FailedJobStatus
	notifier.expectedState.Apply.EndTime = endTime

	err = subject.UpdateApplyJobWithStatus(state.FailedJobStatus, state.UpdateOptions{
		EndTime: endTime,
	})

	assert.NoError(t, err)
	assert.True(t, notifier.called)
}

func TestUpdatePlanJob(t *testing.T) {
	route := &mux.Route{}
	route.Path("/jobs/{job-id}")

	exoectedURL, err := url.Parse("www.test.com/jobs/1234")
	assert.NoError(t, err)
	deployMode := terraform.Deploy

	jobID := bytes.NewBufferString("1234")
	notifier := &testNotifier{
		expectedState: &state.Workflow{
			Mode: &deployMode,
			Plan: &state.Job{
				Status: state.WaitingJobStatus,
				Output: &state.JobOutput{
					URL: exoectedURL,
				},
				ID: jobID.String(),
			},
			ID: workflowID,
		},
		t: t,
	}

	subject := state.NewWorkflowStore(notifier.notify, terraform.Deploy, workflowID)

	baseURL := bytes.NewBufferString("www.test.com")

	// init and then update
	err = subject.InitPlanJob(jobID, baseURL)
	assert.NoError(t, err)

	notifier.expectedState.Plan.Status = state.FailedJobStatus

	err = subject.UpdatePlanJobWithStatus(state.FailedJobStatus)
	assert.NoError(t, err)
	assert.True(t, notifier.called)
}

func TestInitValidateJob(t *testing.T) {
	expectedURL, err := url.Parse("www.test.com/jobs/1234")
	assert.NoError(t, err)
	prMode := terraform.PR

	jobID := bytes.NewBufferString("1234")
	notifier := &testNotifier{
		expectedState: &state.Workflow{
			Mode: &prMode,
			Validate: &state.Job{
				Status: state.WaitingJobStatus,
				Output: &state.JobOutput{
					URL: expectedURL,
				},
				ID: jobID.String(),
			},
			ID: workflowID,
		},
		t: t,
	}

	subject := state.NewWorkflowStore(notifier.notify, terraform.PR, workflowID)

	baseURL := bytes.NewBufferString("www.test.com")

	err = subject.InitValidateJob(jobID, baseURL)
	assert.NoError(t, err)
	assert.True(t, notifier.called)
}

func TestUpdateValidateJob(t *testing.T) {
	route := &mux.Route{}
	route.Path("/jobs/{job-id}")

	expectedURL, err := url.Parse("www.test.com/jobs/1234")
	assert.NoError(t, err)
	prMode := terraform.PR

	jobID := bytes.NewBufferString("1234")
	notifier := &testNotifier{
		expectedState: &state.Workflow{
			Mode: &prMode,
			Validate: &state.Job{
				Status: state.WaitingJobStatus,
				Output: &state.JobOutput{
					URL: expectedURL,
				},
				ID: jobID.String(),
			},
			ID: workflowID,
		},
		t: t,
	}

	subject := state.NewWorkflowStore(notifier.notify, terraform.PR, workflowID)

	baseURL := bytes.NewBufferString("www.test.com")

	// init and then update
	err = subject.InitValidateJob(jobID, baseURL)
	assert.NoError(t, err)

	notifier.expectedState.Validate.Status = state.FailedJobStatus

	err = subject.UpdateValidateJobWithStatus(state.FailedJobStatus)
	assert.NoError(t, err)
	assert.True(t, notifier.called)
}
