package temporalworker

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/neptune/lyft/locksweeper"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

// newLockSweeperActivities returns nil when the sweeper is not enabled.
func newLockSweeperActivities(cfg valid.LockSweeper, temporalClient client.Client, namespace string) (*locksweeper.Activities, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	sess, err := session.NewSession()
	if err != nil {
		return nil, errors.Wrap(err, "creating AWS session for the lock sweeper")
	}
	return &locksweeper.Activities{
		Config: locksweeper.Config{
			Bucket:      cfg.Bucket,
			KeyPrefix:   cfg.KeyPrefix,
			Repository:  cfg.Repository,
			Table:       cfg.Table,
			OwnerPrefix: cfg.OwnerPrefix,
			MinAge:      time.Duration(cfg.MinAgeMinutes) * time.Minute,
			DryRun:      cfg.DryRun,
		},
		S3:        s3.New(sess, aws.NewConfig()),
		Dynamo:    dynamodb.New(sess, aws.NewConfig()),
		Temporal:  temporalClient,
		Namespace: namespace,
	}, nil
}

// startLockSweeper starts the cron-scheduled sweeper workflow. The fixed workflow ID makes this
// idempotent across pods and restarts: only one schedule exists cluster-wide.
func (s Server) startLockSweeper(ctx context.Context) {
	if s.LockSweeper == nil {
		return
	}
	_, err := s.TemporalClient.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:           locksweeper.WorkflowID,
		TaskQueue:    workflows.DeployTaskQueue,
		CronSchedule: locksweeper.CronSchedule,
	}, locksweeper.Workflow)
	if err != nil {
		if _, ok := err.(*serviceerror.WorkflowExecutionAlreadyStarted); ok {
			s.Logger.InfoContext(ctx, "lock sweeper workflow is already scheduled")
			return
		}
		s.Logger.ErrorContext(ctx, "unable to start the lock sweeper workflow: "+err.Error())
		return
	}
	s.Logger.InfoContext(ctx, "scheduled the lock sweeper workflow")
}
