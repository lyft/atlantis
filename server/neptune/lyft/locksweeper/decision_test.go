package locksweeper

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDecide(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	minAge := 90 * time.Minute
	old := now.Add(-3 * time.Hour)
	const atlantis = "atlantis-svc@worker-0"

	cases := []struct {
		name string
		lock Lock
		busy bool
		want Action
	}{
		{"old plan lock is deleted", Lock{Who: atlantis, Operation: PlanOperation, Created: old}, false, Delete},
		{"old apply lock alerts", Lock{Who: atlantis, Operation: "OperationTypeApply", Created: old}, false, Alert},
		{"unknown operation alerts", Lock{Who: atlantis, Operation: "OperationTypeRefresh", Created: old}, false, Alert},
		{"young lock is skipped", Lock{Who: atlantis, Operation: PlanOperation, Created: now.Add(-10 * time.Minute)}, false, Skip},
		{"exactly at the threshold is deleted", Lock{Who: atlantis, Operation: PlanOperation, Created: now.Add(-minAge)}, false, Delete},
		{"busy root is skipped", Lock{Who: atlantis, Operation: PlanOperation, Created: old}, true, Skip},
		{"busy root apply is skipped, not alerted", Lock{Who: atlantis, Operation: "OperationTypeApply", Created: old}, true, Skip},
		{"old plan lock held by a person alerts", Lock{Who: "jdoe@laptop", Operation: PlanOperation, Created: old}, false, Alert},
		{"unknown age is skipped", Lock{Who: atlantis, Operation: PlanOperation}, false, Skip},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Decide(tc.lock, now, minAge, "atlantis-svc@worker", tc.busy).Action)
		})
	}
}
