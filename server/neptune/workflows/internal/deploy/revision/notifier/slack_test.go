package notifier_test

import (
	"context"
	"testing"
	"time"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	terraformActivities "github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/lock"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision/notifier"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision/queue"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type request struct {
	LockState    lock.LockState
	InitialItems []terraform.DeploymentInfo
}

type testActivities struct{}

func (a *testActivities) MessageChannel(ctx context.Context, request activities.MessageChannelRequest) (activities.MessageChannelResponse, error) {
	return activities.MessageChannelResponse{MessageID: "123"}, nil
}

func testWorkflow(ctx workflow.Context, request request) error {
	ctx = workflow.WithScheduleToCloseTimeout(ctx, time.Minute)
	q := queue.NewQueue(func(ctx workflow.Context, d *queue.Deploy) {}, metrics.NewNullableScope())

	for _, i := range request.InitialItems {
		q.Push(i)
	}

	q.SetLockForMergedItems(ctx, request.LockState)

	var a *testActivities
	subject := notifier.Slack{
		DeployQueue: q,
		Activities:  a,
	}

	return subject.Notify(ctx)
}

func TestNotifier(t *testing.T) {
	t.Run("empty queue", func(t *testing.T) {
		state := lock.LockState{Status: lock.UnlockedStatus}
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()

		da := &testActivities{}
		env.RegisterActivity(da)

		env.OnActivity(da.MessageChannel).Never()
		env.ExecuteWorkflow(testWorkflow, request{
			LockState: state,
		})
		err := env.GetWorkflowResult(nil)
		assert.NoError(t, err)
		assert.True(t, env.AssertExpectations(t))
	})

	t.Run("locked state", func(t *testing.T) {
		state := lock.LockState{Status: lock.LockedStatus}
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()

		da := &testActivities{}
		env.RegisterActivity(da)

		env.OnActivity(da.MessageChannel).Never()
		env.ExecuteWorkflow(testWorkflow, request{
			LockState: state,
		})
		err := env.GetWorkflowResult(nil)
		assert.NoError(t, err)
		assert.True(t, env.AssertExpectations(t))
	})

	t.Run("no slack config", func(t *testing.T) {
		state := lock.LockState{Status: lock.LockedStatus}
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()

		da := &testActivities{}
		env.RegisterActivity(da)

		env.OnActivity(da.MessageChannel).Never()
		env.ExecuteWorkflow(testWorkflow, request{
			LockState: state,
			InitialItems: []terraform.DeploymentInfo{
				{
					Repo: github.Repo{
						Name:  "repo",
						Owner: "nish",
					},
					Root: terraformActivities.Root{
						Name: "some-root",
						TriggerInfo: terraformActivities.TriggerInfo{
							Type: terraformActivities.MergeTrigger,
						},
					},
					Commit: github.Commit{
						Revision: "5678",
					},
				},
			},
		})

		err := env.GetWorkflowResult(nil)
		assert.NoError(t, err)

		assert.True(t, env.AssertExpectations(t))
	})

	t.Run("activity called", func(t *testing.T) {
		state := lock.LockState{Status: lock.LockedStatus}
		ts := testsuite.WorkflowTestSuite{}
		env := ts.NewTestWorkflowEnvironment()

		da := &testActivities{}
		env.RegisterActivity(da)

		env.OnActivity(da.MessageChannel, mock.Anything, activities.MessageChannelRequest{
			ChannelID: "1234",
			Message:   "Deploys are locked for *some-root* in *nish/repo*.  Please navigate to the revision's check run to unlock the root",
			Attachments: []slack.Attachment{
				{
					Title: " ",       // purposely empty
					Color: "#36a64f", // green
					Actions: []slack.AttachmentAction{
						{
							Type: "button",
							Text: "Open Revision",
							URL:  "[5678](https://github.com/nish/repo/commit/5678)",
						},
					},
				},
			},
		}).Return(activities.MessageChannelResponse{MessageID: "123"}, nil)
		env.ExecuteWorkflow(testWorkflow, request{
			LockState: state,
			InitialItems: []terraform.DeploymentInfo{
				{
					Repo: github.Repo{
						Name:  "repo",
						Owner: "nish",
					},
					Root: terraformActivities.Root{
						Name: "some-root",
						TriggerInfo: terraformActivities.TriggerInfo{
							Type: terraformActivities.MergeTrigger,
						},
					},
					Commit: github.Commit{
						Revision: "5678",
					},

					Notifications: terraform.NotificationConfig{
						Slack: terraform.SlackConfig{
							ChannelID: "1234",
						},
					},
				},
			},
		})

		err := env.GetWorkflowResult(nil)
		assert.NoError(t, err)

		assert.True(t, env.AssertExpectations(t))
	})
}
