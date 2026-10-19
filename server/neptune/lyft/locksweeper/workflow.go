package locksweeper

import (
	"time"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/lyft/workflows/metrics"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	// WorkflowID is fixed so a cron-scheduled run can only exist once cluster-wide.
	WorkflowID = "terraform-lock-sweeper"
	// CronSchedule runs the sweeper every 30 minutes.
	CronSchedule = "*/30 * * * *"

	startToCloseTimeout = 5 * time.Minute
	retryCount          = 3
)

// Workflow lists the Terraform state locks, and for each one old enough to be orphaned either
// deletes it (plan locks), alerts on it (everything else), or leaves it. With Config.DryRun it only
// reports what it would do.
func Workflow(ctx workflow.Context) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: startToCloseTimeout,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: retryCount},
	})
	scope := metrics.NewScope(ctx, "locksweeper")
	var a *Activities
	now := workflow.Now(ctx)

	var cfg Config
	if err := workflow.ExecuteActivity(ctx, a.GetConfig).Get(ctx, &cfg); err != nil {
		return errors.Wrap(err, "getting config")
	}

	var listed ListLocksResponse
	err := workflow.ExecuteActivity(ctx, a.ListLocks, ListLocksRequest{
		Bucket:    cfg.Bucket,
		KeyPrefix: cfg.KeyPrefix,
		Table:     cfg.Table,
	}).Get(ctx, &listed)
	if err != nil {
		return errors.Wrap(err, "listing locks")
	}

	logger := workflow.GetLogger(ctx)
	var oldest time.Duration
	busy := map[string]bool{}

	for _, lock := range listed.Locks {
		if !lock.Created.IsZero() && now.Sub(lock.Created) > oldest {
			oldest = now.Sub(lock.Created)
		}

		// Only ask Temporal about locks that are already old enough to matter.
		rootBusy := false
		if !lock.Created.IsZero() && now.Sub(lock.Created) >= cfg.MinAge {
			b, ok := busy[lock.Root]
			if !ok {
				var resp RootBusyResponse
				if err := workflow.ExecuteActivity(ctx, a.RootBusy, RootBusyRequest{
					Repository: cfg.Repository,
					Root:       lock.Root,
				}).Get(ctx, &resp); err != nil {
					// Fail safe: if we cannot tell whether the root is running, leave the lock alone.
					logger.Warn("could not check Temporal for the root, skipping its lock", "root", lock.Root, "store", string(lock.Store), "error", err.Error())
					scope.Counter("check_failed").Inc(1)
					continue
				}
				b = resp.Busy
				busy[lock.Root] = b
			}
			rootBusy = b
		}

		decision := Decide(lock, now, cfg.MinAge, cfg.OwnerPrefix, rootBusy)
		switch decision.Action {
		case Skip:
			scope.Counter("skipped").Inc(1)
		case Alert:
			scope.Counter("alert").Inc(1)
			logger.Error("orphaned Terraform lock needs a manual check",
				"root", lock.Root, "store", string(lock.Store), "operation", lock.Operation,
				"who", lock.Who, "created", lock.Created.Format(time.RFC3339), "reason", decision.Reason)
		case Delete:
			if cfg.DryRun {
				scope.Counter("would_delete").Inc(1)
				logger.Info("dry run: would delete orphaned Terraform lock",
					"root", lock.Root, "store", string(lock.Store), "reason", decision.Reason)
				continue
			}
			var resp DeleteLockResponse
			if err := workflow.ExecuteActivity(ctx, a.DeleteLock, DeleteLockRequest{
				Bucket: cfg.Bucket,
				Table:  cfg.Table,
				Lock:   lock,
			}).Get(ctx, &resp); err != nil {
				logger.Error("failed to delete orphaned Terraform lock", "root", lock.Root, "store", string(lock.Store), "error", err.Error())
				scope.Counter("delete_failed").Inc(1)
				continue
			}
			if resp.Deleted {
				scope.Counter("deleted").Inc(1)
				logger.Info("deleted orphaned Terraform lock", "root", lock.Root, "store", string(lock.Store), "reason", decision.Reason)
			} else {
				scope.Counter("changed_before_delete").Inc(1)
			}
		}
	}

	scope.Gauge("oldest_lock_age_minutes").Update(oldest.Minutes())
	return nil
}
