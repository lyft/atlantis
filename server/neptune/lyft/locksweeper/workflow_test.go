package locksweeper_test

import (
	"context"
	"testing"
	"time"

	"github.com/runatlantis/atlantis/server/neptune/lyft/locksweeper"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

func run(t *testing.T, now time.Time, cfg locksweeper.Config, locks []locksweeper.Lock, busyRoots map[string]bool, onDelete func(locksweeper.Lock)) {
	t.Helper()
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	env.SetStartTime(now)
	var a *locksweeper.Activities
	env.RegisterActivity(a)

	env.OnActivity(a.GetConfig, mock.Anything).Return(cfg, nil)
	env.OnActivity(a.ListLocks, mock.Anything, mock.Anything).Return(locksweeper.ListLocksResponse{Locks: locks}, nil)
	env.OnActivity(a.RootBusy, mock.Anything, mock.Anything).Return(
		func(_ context.Context, req locksweeper.RootBusyRequest) (locksweeper.RootBusyResponse, error) {
			return locksweeper.RootBusyResponse{Busy: busyRoots[req.Root]}, nil
		})
	env.OnActivity(a.DeleteLock, mock.Anything, mock.Anything).Return(
		func(_ context.Context, req locksweeper.DeleteLockRequest) (locksweeper.DeleteLockResponse, error) {
			onDelete(req.Lock)
			return locksweeper.DeleteLockResponse{Deleted: true}, nil
		})

	env.ExecuteWorkflow(locksweeper.Workflow)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
}

func TestWorkflow(t *testing.T) {
	cfg := locksweeper.Config{
		Bucket: "b", KeyPrefix: "lyft/dsterraform", Repository: "lyft/dsterraform", Table: "t",
		MinAge: 90 * time.Minute, OwnerPrefix: "atlantis-svc@worker",
	}
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	old := now.Add(-5 * time.Hour)
	young := now.Add(-5 * time.Minute)

	locks := []locksweeper.Lock{
		{Store: locksweeper.StoreS3, Root: "plan-old", Who: "atlantis-svc@worker-1", Operation: locksweeper.PlanOperation, Created: old},
		{Store: locksweeper.StoreDynamoDB, Root: "plan-old", Who: "atlantis-svc@worker-1", Operation: locksweeper.PlanOperation, Created: old},
		{Store: locksweeper.StoreS3, Root: "apply-old", Who: "atlantis-svc@worker-1", Operation: "OperationTypeApply", Created: old},
		{Store: locksweeper.StoreS3, Root: "plan-young", Who: "atlantis-svc@worker-1", Operation: locksweeper.PlanOperation, Created: young},
		{Store: locksweeper.StoreS3, Root: "plan-human", Who: "jdoe@laptop", Operation: locksweeper.PlanOperation, Created: old},
		{Store: locksweeper.StoreS3, Root: "plan-busy", Who: "atlantis-svc@worker-1", Operation: locksweeper.PlanOperation, Created: old},
	}
	busy := map[string]bool{"plan-busy": true}

	t.Run("deletes only old idle plan locks", func(t *testing.T) {
		var deleted []string
		run(t, now, cfg, locks, busy, func(l locksweeper.Lock) { deleted = append(deleted, string(l.Store)+"/"+l.Root) })
		require.ElementsMatch(t, []string{"s3/plan-old", "dynamodb/plan-old"}, deleted)
	})

	t.Run("dry run deletes nothing", func(t *testing.T) {
		dry := cfg
		dry.DryRun = true
		run(t, now, dry, locks, busy, func(l locksweeper.Lock) { t.Fatalf("dry run deleted %s/%s", l.Store, l.Root) })
	})
}
