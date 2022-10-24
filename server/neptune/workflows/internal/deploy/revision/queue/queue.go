package queue

import (
	"container/list"
	"fmt"

	activity "github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
)

type LockStatus int

const (
	UnlockedStatus LockStatus = iota
	LockedStatus
)

type Deploy struct {
	queue *priority

	// mutable: default is unlocked
	lock LockStatus
}

func NewQueue() *Deploy {
	return &Deploy{
		queue: newPriorityQueue(),
	}

}

func (q *Deploy) SetLockStatusForMergedTrigger(status LockStatus) {
	q.lock = status
}

func (q *Deploy) CanPop() bool {
	return q.queue.HasItemsOfPriority(High) || (q.lock == UnlockedStatus && !q.queue.IsEmpty())
}

func (q *Deploy) Pop() (terraform.DeploymentInfo, error) {
	return q.queue.Pop()
}

func (q *Deploy) IsEmpty() bool {
	return q.queue.IsEmpty()
}

func (q *Deploy) Push(msg terraform.DeploymentInfo) {
	if msg.Root.Trigger == activity.ManualTrigger {
		q.queue.Push(msg, High)
		return
	}

	q.queue.Push(msg, Low)
}

// priority is a simple 2 priority queue implementation
// priority is determined before an item enters a queue and does not change
type priority struct {
	queues map[priorityType]*list.List
}

type priorityType int

const (
	Low priorityType = iota + 1
	High
)

func newPriorityQueue() *priority {
	return &priority{
		queues: map[priorityType]*list.List{
			High: list.New(),
			Low:  list.New(),
		},
	}
}

func (q *priority) IsEmpty() bool {
	for _, q := range q.queues {
		if q.Len() != 0 {
			return false
		}
	}
	return true
}

func (q *priority) HasItemsOfPriority(priority priorityType) bool {
	return q.queues[priority].Len() != 0
}

func (q *priority) Push(msg terraform.DeploymentInfo, priority priorityType) {
	q.queues[priority].PushBack(msg)
}

func (q *priority) Pop() (terraform.DeploymentInfo, error) {
	priority := High
	if q.queues[High].Len() == 0 {
		priority = Low
	}

	if q.queues[priority].Len() == 0 {
		return terraform.DeploymentInfo{}, fmt.Errorf("no items to pop")
	}

	result := q.queues[priority].Remove(q.queues[priority].Front())
	// naughty casting
	return result.(terraform.DeploymentInfo), nil
}
