package signals

import (
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

func NewRevisionSignal(ctx workflow.Context, queue Queue) *NewRevision {
	return &NewRevision{
		input: workflow.GetSignalChannel(ctx, NewRevisionID),
		queue: queue,
	}
}

type NewRevision struct {
	input workflow.ReceiveChannel
	queue Queue
}

func (n *NewRevision) AddCallback(ctx workflow.Context, selector workflow.Selector) {
	selector.AddReceive(n.input, func(c workflow.ReceiveChannel, more bool) {

		var request NewRevisionRequest
		c.Receive(ctx, &request)

		n.queue.Push(queue.Message{
			Revision: request.Revision,
		})
	})
}
