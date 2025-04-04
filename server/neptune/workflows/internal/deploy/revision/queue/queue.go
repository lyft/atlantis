package queue

import (
	"container/list"
	"fmt"
	"strings"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	activity "github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/lock"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"go.temporal.io/sdk/workflow"
)

type Deploy struct {
	queue              *priority
	lockStatusCallback func(workflow.Context, *Deploy)
	scope              metrics.Scope

	// mutable: default is unlocked
	lock lock.LockState
}

func NewQueue(callback func(workflow.Context, *Deploy), scope metrics.Scope) *Deploy {
	return &Deploy{
		queue:              newPriorityQueue(),
		lockStatusCallback: callback,
		scope:              scope,
	}
}

func (q *Deploy) GetLockState() lock.LockState {
	return q.lock
}

func (q *Deploy) SetLockForMergedItems(ctx workflow.Context, state lock.LockState) {
	if state.Status == lock.LockedStatus {
		q.scope.Counter("locked").Inc(1)
	} else {
		q.scope.Counter("unlocked").Inc(1)
	}
	q.lock = state
	q.lockStatusCallback(ctx, q)
}

func (q *Deploy) CanPop() bool {
	return q.queue.HasItemsOfPriority(High) || (q.lock.Status == lock.UnlockedStatus && !q.queue.IsEmpty())
}

func (q *Deploy) Pop() (terraform.DeploymentInfo, error) {
	defer q.scope.Gauge(lock.QueueDepthStat).Update(float64(q.queue.Size()))
	return q.queue.Pop()
}

func (q *Deploy) Scan() []terraform.DeploymentInfo {
	return append(q.queue.Scan(High), q.queue.Scan(Low)...)
}

func (q *Deploy) GetOrderedMergedItems() []terraform.DeploymentInfo {
	return q.queue.Scan(Low)
}

func (q *Deploy) IsEmpty() bool {
	return q.queue.IsEmpty()
}

func (q *Deploy) Push(msg terraform.DeploymentInfo) {
	defer q.scope.Gauge(lock.QueueDepthStat).Update(float64(q.queue.Size()))
	if msg.Root.TriggerInfo.Type == activity.ManualTrigger {
		q.queue.Push(msg, High)
		return
	}
	q.queue.Push(msg, Low)
}

func (q *Deploy) GetQueuedRevisionsSummary() string {
	var revisions []string
	if q.IsEmpty() {
		return "No other runs ahead in queue."
	}
	for _, deploy := range q.Scan() {
		runLink := github.BuildRunURLMarkdown(deploy.Repo.GetFullName(), deploy.Commit.Revision, deploy.CheckRunID)
		revisions = append(revisions, runLink)
	}
	return fmt.Sprintf("Runs in queue: %s", strings.Join(revisions, ", "))
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

func (q *priority) Size() int {
	size := 0
	for _, queue := range q.queues {
		size += queue.Len()
	}
	return size
}

func (q *priority) Scan(priority priorityType) []terraform.DeploymentInfo {
	var result []terraform.DeploymentInfo

	for e := q.queues[priority].Front(); e != nil; e = e.Next() {
		result = append(result, e.Value.(terraform.DeploymentInfo))
	}

	return result
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
