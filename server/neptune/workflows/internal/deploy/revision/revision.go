package revision

import (
	"context"

	"github.com/google/uuid"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	terraformActivity "github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/config/logger"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/request"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/request/converter"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision/queue"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type idGenerator func(ctx workflow.Context) (uuid.UUID, error)

type proxySignaler interface {
	SignalProxyWorkflow(ctx workflow.Context, msg terraform.DeploymentInfo) error
}

type queueLock interface {
	SetStatus(status queue.LockStatus)
}

type NewRevisionRequest struct {
	Revision       string
	InitiatingUser request.User
	Root           request.Root
	Repo           request.Repo
	Tags           map[string]string
}

type Queue interface {
	Push(terraform.DeploymentInfo)
}

type Activities interface {
	CreateCheckRun(ctx context.Context, request activities.CreateCheckRunRequest) (activities.CreateCheckRunResponse, error)
}

func NewReceiver(
	ctx workflow.Context,
	queue Queue,
	activities Activities,
	generator idGenerator,
	proxySignaler proxySignaler,
	queueLock queueLock,
) *Receiver {
	return &Receiver{
		queue:         queue,
		queueLock:     queueLock,
		ctx:           ctx,
		activities:    activities,
		idGenerator:   generator,
		proxySignaler: proxySignaler,
	}
}

type Receiver struct {
	queue         Queue
	queueLock     queueLock
	ctx           workflow.Context
	activities    Activities
	idGenerator   idGenerator
	proxySignaler proxySignaler
}

func (n *Receiver) Receive(c workflow.ReceiveChannel, more bool) {
	// more is false when the channel is closed, so let's just return right away
	if !more {
		return
	}

	var request NewRevisionRequest
	c.Receive(n.ctx, &request)

	root := converter.Root(request.Root)
	repo := converter.Repo(request.Repo)
	initiatingUser := converter.User(request.InitiatingUser)

	ctx := workflow.WithRetryPolicy(n.ctx, temporal.RetryPolicy{
		MaximumAttempts: 5,
	})

	// generate an id for this deployment and pass that to our check run
	id, err := n.idGenerator(ctx)

	if err != nil {
		logger.Error(ctx, "generating deployment id", "err", err)
	}

	var resp activities.CreateCheckRunResponse
	err = workflow.ExecuteActivity(ctx, n.activities.CreateCheckRun, activities.CreateCheckRunRequest{
		Title:      terraform.BuildCheckRunTitle(root.Name),
		Sha:        request.Revision,
		Repo:       repo,
		ExternalID: id.String(),
	}).Get(ctx, &resp)

	// don't block on error here, we'll just try again later when we have our result.
	if err != nil {
		logger.Error(ctx, err.Error())
	}

	msg := terraform.DeploymentInfo{
		ID:             id,
		Root:           root,
		Revision:       request.Revision,
		InitiatingUser: initiatingUser,
		CheckRunID:     resp.ID,
		Repo:           repo,
	}

	// hijack manual deployments and directly push them to the terraform worklfow
	if root.Trigger == terraformActivity.ManualTrigger {
		err := n.proxySignaler.SignalProxyWorkflow(ctx, msg)

		if err != nil {
			logger.Error(ctx, "signaling proxy workflow, retry the deployment to proceed.")
		}
	}

	n.queue.Push(msg)
}
