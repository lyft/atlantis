package lock

type LockStatus int

type LockState struct {
	Revision string
	Status   LockStatus
}

const (
	UnlockedStatus LockStatus = iota
	LockedStatus

	QueueDepthStat = "queue.depth"
)
