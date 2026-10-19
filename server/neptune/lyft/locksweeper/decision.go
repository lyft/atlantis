package locksweeper

import (
	"fmt"
	"strings"
	"time"
)

type Action int

const (
	// Skip leaves the lock alone.
	Skip Action = iota
	// Delete removes the lock.
	Delete
	// Alert leaves the lock alone but reports it, because deleting it needs a person to look first.
	Alert
)

type Decision struct {
	Action Action
	Reason string
}

// Decide says what to do with a lock. rootBusy is whether Temporal has an open Terraform workflow
// for the lock's root.
func Decide(l Lock, now time.Time, minAge time.Duration, ownerPrefix string, rootBusy bool) Decision {
	if l.Created.IsZero() {
		return Decision{Skip, "lock age is unknown"}
	}
	age := now.Sub(l.Created)
	if age < minAge {
		return Decision{Skip, fmt.Sprintf("lock is %s old, younger than %s", age.Round(time.Minute), minAge)}
	}
	if rootBusy {
		return Decision{Skip, "an open Terraform workflow exists for this root"}
	}
	if ownerPrefix != "" && !strings.HasPrefix(l.Who, ownerPrefix) {
		return Decision{Alert, fmt.Sprintf("lock is held by %q, not by Atlantis, so it is not deleted automatically", l.Who)}
	}
	if l.Operation != PlanOperation {
		return Decision{Alert, fmt.Sprintf("orphaned %s lock needs a manual check before deleting", l.Operation)}
	}
	return Decision{Delete, fmt.Sprintf("orphaned plan lock, %s old, no open Terraform workflow", age.Round(time.Minute))}
}
