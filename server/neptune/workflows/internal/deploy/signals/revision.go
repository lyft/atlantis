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

func NewRevisionSignalReceiver(ctx workflow.Context, queue Queue) *RevisionReceiver {
	return &RevisionReceiver{
		input: workflow.GetSignalChannel(ctx, NewRevisionID),
		queue: queue,
	}
}

type RevisionReceiver struct {
	input           workflow.ReceiveChannel
	queue           Queue

	// mutable
	timerCancel workflow.CancelFunc
	timeout     bool
}

func (n *RevisionReceiver) AddReceiveWithTimeout(ctx workflow.Context, selector workflow.Selector, timeout time.Duration) {
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
		n.AddTimeout(ctx, selector, timeout)
	})

	n.AddTimeout(ctx, selector, timeout)
}

func (n *RevisionReceiver) AddTimeout(ctx workflow.Context, selector workflow.Selector, timeout time.Duration) {
	n.timeout = false
	ctx, cancel := workflow.WithCancel(ctx)
	selector.AddFuture(workflow.NewTimer(ctx, timeout), func(f workflow.Future) {

		// if canceled we shouldn't do anything
		if ctx.Err() == workflow.ErrCanceled {
			return
		}

		n.timeout = true
	})

	n.timerCancel = cancel
}

func (n *RevisionReceiver) DidTimeout() bool {
	return n.timeout
}
