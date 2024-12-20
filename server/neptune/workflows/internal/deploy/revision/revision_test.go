package revision_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/notifier"

	"github.com/google/uuid"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/lock"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/request"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision/queue"
	terraformWorkflow "github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type testCheckRunClient struct {
	expectedRequest notifier.GithubCheckRunRequest
	expectedT       *testing.T
}

func (t *testCheckRunClient) CreateOrUpdate(ctx workflow.Context, deploymentID string, request notifier.GithubCheckRunRequest) (int64, error) {
	ok := assert.Equal(t.expectedT, t.expectedRequest, request)
	if !ok {
		t.expectedT.FailNow()
	}
	return 1, nil
}

type testQueue struct {
	Queue []terraformWorkflow.DeploymentInfo
	Lock  lock.LockState
}

func (q *testQueue) Scan() []terraformWorkflow.DeploymentInfo {
	return q.Queue
}

func (q *testQueue) Push(msg terraformWorkflow.DeploymentInfo) {
	q.Queue = append(q.Queue, msg)
}

func (q *testQueue) GetLockState() lock.LockState {
	return q.Lock
}

func (q *testQueue) SetLockForMergedItems(ctx workflow.Context, state lock.LockState) {
	q.Lock = state
}

func (q *testQueue) IsEmpty() bool {
	return len(q.Queue) == 0
}

func (q *testQueue) GetQueuedRevisionsSummary() string {
	var revisions []string
	if q.IsEmpty() {
		return "No other revisions ahead In queue."
	}
	for _, deploy := range q.Scan() {
		revisions = append(revisions, deploy.Commit.Revision)
	}
	return fmt.Sprintf("Revisions in queue: %s", strings.Join(revisions, ", "))
}

type testWorker struct {
	Current queue.CurrentDeployment
}

func (t testWorker) GetCurrentDeploymentState() queue.CurrentDeployment {
	return t.Current
}

type req struct {
	ID              uuid.UUID
	Lock            lock.LockState
	Current         queue.CurrentDeployment
	InitialElements []terraformWorkflow.DeploymentInfo
	ExpectedRequest notifier.GithubCheckRunRequest
	ExpectedT       *testing.T
}

type response struct {
	Queue   []terraformWorkflow.DeploymentInfo
	Lock    lock.LockState
	Timeout bool
}

func testWorkflow(ctx workflow.Context, r req) (response, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToCloseTimeout: 5 * time.Second,
	})
	var timeout bool
	queue := &testQueue{
		Lock:  r.Lock,
		Queue: r.InitialElements,
	}

	worker := &testWorker{
		Current: r.Current,
	}

	receiver := revision.NewReceiver(ctx, queue, &testCheckRunClient{
		expectedRequest: r.ExpectedRequest,
		expectedT:       r.ExpectedT,
	}, func(ctx workflow.Context) (uuid.UUID, error) {
		return r.ID, nil
	}, worker)
	selector := workflow.NewSelector(ctx)

	selector.AddReceive(workflow.GetSignalChannel(ctx, "test-signal"), receiver.Receive)

	for {
		selector.Select(ctx)

		if !selector.HasPending() {
			break
		}
	}

	return response{
		Queue:   queue.Queue,
		Lock:    queue.Lock,
		Timeout: timeout,
	}, nil
}

func TestEnqueue(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	rev := "1234"
	branch := "default-branch"

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("test-signal", revision.NewRevisionRequest{
			Revision: rev,
			Branch:   branch,
			Root: request.Root{
				Name: "root",
				TriggerInfo: request.TriggerInfo{
					Type: request.MergeTrigger,
				},
			},
			Repo: request.Repo{Name: "nish"},
		})
	}, 0)

	id := uuid.Must(uuid.NewUUID())

	env.ExecuteWorkflow(testWorkflow, req{
		ID: id,
		ExpectedRequest: notifier.GithubCheckRunRequest{
			Title:   "atlantis/deploy: root",
			Sha:     rev,
			Repo:    github.Repo{Name: "nish"},
			State:   github.CheckRunQueued,
			Summary: "This deploy is queued and will be processed as soon as possible.\nNo other revisions ahead In queue.",
		},
		ExpectedT: t,
	})
	env.AssertExpectations(t)
	assert.True(t, env.IsWorkflowCompleted())

	var resp response
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)

	assert.Equal(t, []terraformWorkflow.DeploymentInfo{
		{
			Commit: github.Commit{
				Revision: rev,
				Branch:   branch,
			},
			CheckRunID: 1,
			Root: terraform.Root{Name: "root", TriggerInfo: terraform.TriggerInfo{
				Type: terraform.MergeTrigger,
			}, Trigger: terraform.MergeTrigger},
			ID:   id,
			Repo: github.Repo{Name: "nish"},
		},
	}, resp.Queue)
	assert.Equal(t, lock.LockState{
		Status: lock.UnlockedStatus,
	}, resp.Lock)
	assert.False(t, resp.Timeout)
}

func TestEnqueue_ManualTrigger(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	rev := "1234"
	branch := "default-branch"

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("test-signal", revision.NewRevisionRequest{
			Revision: rev,
			Branch:   branch,
			Root: request.Root{
				Name: "root",
				TriggerInfo: request.TriggerInfo{
					Type: request.ManualTrigger,
				},
			},
			Repo: request.Repo{Name: "nish"},
		})
	}, 0)

	id := uuid.Must(uuid.NewUUID())

	env.ExecuteWorkflow(testWorkflow, req{
		ID: id,
		ExpectedRequest: notifier.GithubCheckRunRequest{
			Title:   "atlantis/deploy: root",
			Sha:     rev,
			Repo:    github.Repo{Name: "nish"},
			State:   github.CheckRunQueued,
			Summary: "This deploy is queued and will be processed as soon as possible.\nNo other revisions ahead In queue.",
		},
		ExpectedT: t,
	})
	env.AssertExpectations(t)
	assert.True(t, env.IsWorkflowCompleted())

	var resp response
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)

	assert.Equal(t, []terraformWorkflow.DeploymentInfo{
		{
			Commit: github.Commit{
				Revision: rev,
				Branch:   branch,
			},
			CheckRunID: 1,
			Root: terraform.Root{Name: "root", TriggerInfo: terraform.TriggerInfo{
				Type: terraform.ManualTrigger,
			}, Trigger: terraform.ManualTrigger},
			ID:   id,
			Repo: github.Repo{Name: "nish"},
		},
	}, resp.Queue)
	assert.Equal(t, lock.LockState{
		Status:   lock.LockedStatus,
		Revision: "1234",
	}, resp.Lock)
	assert.False(t, resp.Timeout)
}

func TestEnqueue_ManualTrigger_QueueAlreadyLocked(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	rev := "1234"
	branch := "default-branch"

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("test-signal", revision.NewRevisionRequest{
			Revision: rev,
			Branch:   branch,
			Root: request.Root{
				Name: "root",
				TriggerInfo: request.TriggerInfo{
					Type: request.ManualTrigger,
				},
			},
			Repo: request.Repo{Name: "nish"},
		})
	}, 0)

	id := uuid.Must(uuid.NewUUID())

	env.ExecuteWorkflow(testWorkflow, req{
		ID: id,
		Lock: lock.LockState{
			// ensure that the lock gets updated
			Status:   lock.LockedStatus,
			Revision: "123334444555",
		},
		ExpectedRequest: notifier.GithubCheckRunRequest{
			Title:   "atlantis/deploy: root",
			Sha:     rev,
			Repo:    github.Repo{Name: "nish"},
			State:   github.CheckRunQueued,
			Summary: "This deploy is queued and will be processed as soon as possible.\nNo other revisions ahead In queue.",
		},
		ExpectedT: t,
	})
	env.AssertExpectations(t)
	assert.True(t, env.IsWorkflowCompleted())

	var resp response
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)

	assert.Equal(t, []terraformWorkflow.DeploymentInfo{
		{
			Commit: github.Commit{
				Revision: rev,
				Branch:   branch,
			},
			CheckRunID: 1,
			Root: terraform.Root{Name: "root", TriggerInfo: terraform.TriggerInfo{
				Type: terraform.ManualTrigger,
			}, Trigger: terraform.ManualTrigger},
			ID:   id,
			Repo: github.Repo{Name: "nish"},
		},
	}, resp.Queue)
	assert.Equal(t, lock.LockState{
		Status:   lock.LockedStatus,
		Revision: "1234",
	}, resp.Lock)
	assert.False(t, resp.Timeout)
}

func TestEnqueue_MergeTrigger_QueueAlreadyLocked(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	rev := "1234"
	branch := "default-branch"

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("test-signal", revision.NewRevisionRequest{
			Revision: rev,
			Branch:   branch,
			Root: request.Root{
				Name: "root",
				TriggerInfo: request.TriggerInfo{
					Type: request.MergeTrigger,
				},
			},
			Repo: request.Repo{Name: "nish"},
		})
	}, 0)

	id := uuid.Must(uuid.NewUUID())

	deploymentInfo := terraformWorkflow.DeploymentInfo{
		Commit: github.Commit{
			Revision: "123334444555",
			Branch:   "locking-branch",
		},
		CheckRunID: 0,
		Root: terraform.Root{Name: "root", TriggerInfo: terraform.TriggerInfo{
			Type: terraform.MergeTrigger,
		}, Trigger: terraform.MergeTrigger},
		ID:   id,
		Repo: github.Repo{Name: "nish"},
	}

	env.ExecuteWorkflow(testWorkflow, req{
		ID:              id,
		InitialElements: []terraformWorkflow.DeploymentInfo{deploymentInfo},
		Lock: lock.LockState{
			// ensure that the lock gets updated
			Status:   lock.LockedStatus,
			Revision: "123334444555",
		},
		ExpectedRequest: notifier.GithubCheckRunRequest{
			Title:   "atlantis/deploy: root",
			Sha:     rev,
			Repo:    github.Repo{Name: "nish"},
			Summary: "This deploy is locked from a manual deployment for revision [123334444555](https://github.com//nish/commit/123334444555).  Unlock to proceed.\nRevisions in queue: 123334444555",
			Actions: []github.CheckRunAction{github.CreateUnlockAction()},
			State:   github.CheckRunActionRequired,
		},
		ExpectedT: t,
	})
	env.AssertExpectations(t)
	assert.True(t, env.IsWorkflowCompleted())

	var resp response
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)

	assert.Equal(t, []terraformWorkflow.DeploymentInfo{
		deploymentInfo,
		{
			Commit: github.Commit{
				Revision: rev,
				Branch:   branch,
			},
			CheckRunID: 1,
			Root: terraform.Root{Name: "root", TriggerInfo: terraform.TriggerInfo{
				Type: terraform.MergeTrigger,
			}, Trigger: terraform.MergeTrigger},
			ID:   id,
			Repo: github.Repo{Name: "nish"},
		},
	}, resp.Queue)
	assert.Equal(t, lock.LockState{
		Status:   lock.LockedStatus,
		Revision: "123334444555",
	}, resp.Lock)
	assert.False(t, resp.Timeout)
}

func TestEnqueue_ManualTrigger_RequestAlreadyInQueue(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	rev := "1234"
	branch := "default-branch"

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("test-signal", revision.NewRevisionRequest{
			Revision: rev,
			Branch:   branch,
			Root: request.Root{
				Name: "root",
				TriggerInfo: request.TriggerInfo{
					Type: request.ManualTrigger,
				},
			},
			Repo: request.Repo{Name: "nish"},
		})
	}, 0)

	id := uuid.Must(uuid.NewUUID())

	deploymentInfo := terraformWorkflow.DeploymentInfo{
		Commit: github.Commit{
			Revision: rev,
			Branch:   branch,
		},
		CheckRunID: 1,
		Root: terraform.Root{Name: "root", TriggerInfo: terraform.TriggerInfo{
			Type: terraform.ManualTrigger,
		}, Trigger: terraform.ManualTrigger},
		ID:   id,
		Repo: github.Repo{Name: "nish"},
	}
	env.ExecuteWorkflow(testWorkflow, req{
		ID:              id,
		InitialElements: []terraformWorkflow.DeploymentInfo{deploymentInfo},
	})
	env.AssertExpectations(t)
	assert.True(t, env.IsWorkflowCompleted())

	var resp response
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)
	// should not another of the same to the queue
	assert.Equal(t, []terraformWorkflow.DeploymentInfo{deploymentInfo}, resp.Queue)
}

func TestEnqueue_ManualTrigger_RequestAlreadyInProgress(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	rev := "1234"
	branch := "default-branch"

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("test-signal", revision.NewRevisionRequest{
			Revision: rev,
			Branch:   branch,
			Root: request.Root{
				Name: "root",
				TriggerInfo: request.TriggerInfo{
					Type: request.ManualTrigger,
				},
			},
			Repo: request.Repo{Name: "nish"},
		})
	}, 0)

	id := uuid.Must(uuid.NewUUID())

	deploymentInfo := terraformWorkflow.DeploymentInfo{
		Commit: github.Commit{
			Revision: rev,
			Branch:   branch,
		},
		CheckRunID: 1,
		Root: terraform.Root{Name: "root", TriggerInfo: terraform.TriggerInfo{
			Type: terraform.ManualTrigger,
		}, Trigger: terraform.ManualTrigger},
		ID:   id,
		Repo: github.Repo{Name: "nish"},
	}
	env.ExecuteWorkflow(testWorkflow, req{
		ID: id,
		Current: queue.CurrentDeployment{
			Deployment: deploymentInfo,
			Status:     queue.InProgressStatus,
		},
	})
	env.AssertExpectations(t)
	assert.True(t, env.IsWorkflowCompleted())

	var resp response
	err := env.GetWorkflowResult(&resp)
	assert.NoError(t, err)
	// should not add in progress to the queue
	assert.Empty(t, resp.Queue)
}
