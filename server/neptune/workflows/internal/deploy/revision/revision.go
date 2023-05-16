package revision

import (
	"context"
	"fmt"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/notifier"

	"github.com/runatlantis/atlantis/server/events/metrics"
	key "github.com/runatlantis/atlantis/server/neptune/context"

	"github.com/google/uuid"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	activity "github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/request"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/request/converter"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision/queue"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	// signals
	NewRevisionSignalID = "new-revision"
)

type CheckRunClient interface {
	CreateOrUpdate(ctx workflow.Context, deploymentID string, request notifier.GithubCheckRunRequest) (int64, error)
}

type idGenerator func(ctx workflow.Context) (uuid.UUID, error)

type NewRevisionRequest struct {
	Revision       string
	Branch         string
	InitiatingUser request.User
	Root           request.Root
	Repo           request.Repo
	Tags           map[string]string
}

type Queue interface {
	Push(terraform.DeploymentInfo)
	GetLockState() queue.LockState
	SetLockForMergedItems(ctx workflow.Context, state queue.LockState)
	Scan() []terraform.DeploymentInfo
}

type DeploymentStore interface {
	GetCurrentDeploymentState() queue.CurrentDeployment
}

type Activities interface {
	GithubCreateCheckRun(ctx context.Context, request activities.CreateCheckRunRequest) (activities.CreateCheckRunResponse, error)
}

func NewReceiver(ctx workflow.Context, queue Queue, checkRunClient CheckRunClient, generator idGenerator, worker DeploymentStore) *Receiver {
	return &Receiver{
		queue:          queue,
		ctx:            ctx,
		idGenerator:    generator,
		worker:         worker,
		checkRunClient: checkRunClient,
	}
}

type Receiver struct {
	queue          Queue
	ctx            workflow.Context
	idGenerator    idGenerator
	worker         DeploymentStore
	checkRunClient CheckRunClient
}

func (n *Receiver) Receive(c workflow.ReceiveChannel, more bool) {
	// more is false when the channel is closed, so let's just return right away
	if !more {
		return
	}

	var request NewRevisionRequest
	c.Receive(n.ctx, &request)

	workflow.GetMetricsHandler(n.ctx).WithTags(map[string]string{
		metrics.SignalNameTag: NewRevisionSignalID,
	}).Counter(metrics.SignalReceive).Inc(1)

	root := converter.Root(request.Root)
	repo := converter.Repo(request.Repo)
	initiatingUser := converter.User(request.InitiatingUser)

	ctx := workflow.WithRetryPolicy(n.ctx, temporal.RetryPolicy{
		MaximumAttempts: 5,
	})

	// generate an id for this deployment and pass that to our check run
	id, err := n.idGenerator(ctx)

	if err != nil {
		workflow.GetLogger(ctx).Error("generating deployment id", key.ErrKey, err)
	}

	// Do not push a duplicate/in-progress manual deployment to the queue
	if root.TriggerInfo.Type == activity.ManualTrigger && (n.queueContainsRevision(request.Revision) || n.isInProgress(request.Revision)) {
		//TODO: consider executing a comment activity to notify user
		workflow.GetLogger(ctx).Warn("attempted to perform duplicate manual deploy", "revision", request.Revision)
		return
	}

	checkRunID := n.createCheckRun(ctx, id.String(), request.Revision, root, repo)

	// lock the queue on a manual deployment
	if root.TriggerInfo.Type == activity.ManualTrigger {
		// Lock the queue on a manual deployment
		n.queue.SetLockForMergedItems(ctx, queue.LockState{
			Status:   queue.LockedStatus,
			Revision: request.Revision,
		})
	}
	n.queue.Push(terraform.DeploymentInfo{
		ID:             id,
		Root:           root,
		CheckRunID:     checkRunID,
		Repo:           repo,
		InitiatingUser: initiatingUser,
		Tags:           request.Tags,
		Commit: github.Commit{
			Revision: request.Revision,
			Branch:   request.Branch,
		},
	})
}

func (n *Receiver) createCheckRun(ctx workflow.Context, id, revision string, root activity.Root, repo github.Repo) int64 {
	lock := n.queue.GetLockState()
	var actions []github.CheckRunAction
	summary := "This deploy is queued and will be proceesed as soon as possible."
	state := github.CheckRunQueued

	if lock.Status == queue.LockedStatus && (root.TriggerInfo.Type == activity.MergeTrigger) {
		actions = append(actions, github.CreateUnlockAction())
		state = github.CheckRunActionRequired
		revisionLink := github.BuildRevisionURLMarkdown(repo.GetFullName(), lock.Revision)
		summary = fmt.Sprintf("This deploy is locked from a manual deployment for revision %s.  Unlock to proceed.", revisionLink)
	}

	cid, err := n.checkRunClient.CreateOrUpdate(ctx, id, notifier.GithubCheckRunRequest{
		Title:   notifier.BuildDeployCheckRunTitle(root.Name),
		Sha:     revision,
		Repo:    repo,
		Summary: summary,
		Actions: actions,
		State:   state,
	})

	// don't block on error here, we'll just try again later when we have our result.
	if err != nil {
		workflow.GetLogger(ctx).Error(err.Error())
	}

	return cid
}

func (n *Receiver) isInProgress(revision string) bool {
	current := n.worker.GetCurrentDeploymentState()
	return revision == current.Deployment.Commit.Revision && current.Status == queue.InProgressStatus
}

func (n *Receiver) queueContainsRevision(revision string) bool {
	for _, deployment := range n.queue.Scan() {
		if deployment.Root.TriggerInfo.Type == activity.ManualTrigger && revision == deployment.Commit.Revision {
			return true
		}
	}
	return false
}
