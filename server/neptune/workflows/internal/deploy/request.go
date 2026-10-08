package deploy

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/lock"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
)

type Request struct {
	Repo Repo
	Root Root

	// ContinuedState is set only when the workflow continues as new, and holds
	// the state the previous run hadn't finished with.
	ContinuedState *ContinuedState
}

// Repo Names and Root Names are assumed to be static throughout the lifetime
// of a workflow since that's what our ID is based on
//
// We need these values at minimum during startup atm.
type Repo struct {
	FullName string
}

type Root struct {
	Name string
}

// ContinuedState is the deploy queue's in-memory state, carried into the next
// run when the workflow continues as new. None of it is persisted elsewhere.
type ContinuedState struct {
	// Queue holds queued deployments in the order they'd be popped.
	Queue []terraform.DeploymentInfo
	Lock  lock.LockState
	// CheckRuns maps check run cache keys to check run IDs, so queued
	// deployments keep updating their existing check runs.
	CheckRuns map[string]int64
}
