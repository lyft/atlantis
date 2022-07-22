package signals

import (
	"time"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision/queue"
	"go.temporal.io/sdk/workflow"
)

const NewRevisionID = "new-revision"

type NewRevisionRequest struct {
	Revision string
}

type Queue interface {
	Push(queue.Message)
}

func NewRevisionSignal(ctx workflow.Context, queue Queue, timeout time.Duration) *NewRevision {
	return &NewRevision{
		input:           workflow.GetSignalChannel(ctx, NewRevisionID),
		queue:           queue,
		timeoutDuration: timeout,
	}
}

type NewRevision struct {
	input           workflow.ReceiveChannel
	queue           Queue
	timeoutDuration time.Duration

	// mutable
	timerCancel workflow.CancelFunc
	timeout     bool
}

func (n *NewRevision) AddCallback(ctx workflow.Context, selector workflow.Selector) {
	selector.AddReceive(n.input, func(c workflow.ReceiveChannel, more bool) {

		// more is false when the channel is closed, so let's just return right away
		if !more {
			return
		}

		var request NewRevisionRequest
		c.Receive(ctx, &request)

		// cancel's the existing timeout timer
		n.timerCancel()

		n.queue.Push(queue.Message{
			Revision: request.Revision,
		})

		// add another timeout since this receiver is called each time the channel has a message
		n.addTimeout(ctx, selector)
	})

	n.addTimeout(ctx, selector)
}

func (n *NewRevision) addTimeout(ctx workflow.Context, selector workflow.Selector) {
	n.timeout = false
	ctx, cancel := workflow.WithCancel(ctx)
	selector.AddFuture(workflow.NewTimer(ctx, n.timeoutDuration), func(f workflow.Future) {

		// if canceled we shouldn't do anything
		if ctx.Err() == workflow.ErrCanceled {
			return
		}

		n.timeout = true
	})

	n.timerCancel = cancel
}
