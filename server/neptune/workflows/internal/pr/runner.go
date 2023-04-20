package pr

import (
	"context"
	"github.com/pkg/errors"
	internalContext "github.com/runatlantis/atlantis/server/neptune/context"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	workflowMetrics "github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/receiver"
	internalTerraform "github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/sideeffect"
	"go.temporal.io/sdk/workflow"
	"sync"
)

type Action int64

const (
	OnNewPRRevision Action = iota
	OnShutdown
)

type RunnerState int64

const (
	Working RunnerState = iota
	Waiting
	Complete
)

type runnerActivities interface {
	GithubCompareCommit(ctx context.Context, request activities.CompareCommitRequest) (activities.CompareCommitResponse, error)
}

type Runner struct {
	RevisionSignalChannel workflow.ReceiveChannel
	RevisionReceiver      *receiver.RevisionReceiver
	ShutdownSignalChannel workflow.ReceiveChannel
	ShutdownReceiver      *receiver.ShutdownReceiver
	Activities            runnerActivities
	Scope                 workflowMetrics.Scope
	TFWorkflowRunner      *internalTerraform.WorkflowRunner

	// mutable state
	state                 RunnerState
	lastProcessedRevision string
}

func newRunner(ctx workflow.Context, scope workflowMetrics.Scope, tfWorkflowRunner *internalTerraform.WorkflowRunner, a *prActivities) *Runner {
	revisionReceiver := receiver.NewRevisionReceiver(ctx, scope)
	shutdownReceiver := receiver.NewShutdownReceiver(ctx, scope)
	return &Runner{
		RevisionSignalChannel: workflow.GetSignalChannel(ctx, receiver.TerraformRevisionSignalID),
		RevisionReceiver:      &revisionReceiver,
		ShutdownSignalChannel: workflow.GetSignalChannel(ctx, receiver.ShutdownSignalID),
		ShutdownReceiver:      &shutdownReceiver,
		Activities:            a,
		Scope:                 scope,
		TFWorkflowRunner:      tfWorkflowRunner,
	}
}

// Run handles managing the workflow's context lifecycles as new signals/poll events are received and
// change the current PRAction status
func (r *Runner) Run(ctx workflow.Context) error {
	var action Action
	var prRevision receiver.Revision

	//TODO: add approve signal, timeouts, poll variations of signals
	s := workflow.NewSelector(ctx)
	s.AddReceive(r.RevisionSignalChannel, func(c workflow.ReceiveChannel, more bool) {
		prRevision = r.RevisionReceiver.Receive(c, more)
		action = OnNewPRRevision
	})

	s.AddReceive(r.ShutdownSignalChannel, func(c workflow.ReceiveChannel, more bool) {
		r.ShutdownReceiver.Receive(c, more)
		action = OnShutdown
		r.state = Complete
	})

	inProgressCtx, inProgressCancel := workflow.WithCancel(ctx)
	for {
		s.Select(ctx)
		switch action {
		case OnNewPRRevision:
			shouldProcess, err := r.shouldProcessRevision(ctx, prRevision)
			if err != nil {
				return err
			}
			if !shouldProcess {
				continue
			}
			// cancel in progress workflow (if it exists) and start up a new one
			inProgressCancel()
			inProgressCtx, inProgressCancel = workflow.WithCancel(ctx)
			inProgressCtx = workflow.WithValue(inProgressCtx, internalContext.SHAKey, prRevision.Revision)
			workflow.GetLogger(inProgressCtx).Info("received revision")
			workflow.Go(inProgressCtx, func(c workflow.Context) {
				r.processRevision(c, prRevision)
			})
		case OnShutdown:
			// shutdown in progress workflow if it exists and return
			workflow.GetLogger(inProgressCtx).Info("received shutdown request")
			inProgressCancel()
			return nil
		}
	}
}

// processRevision handles spinning off child Terraform workflows per root and
// dealing with any failed policies by reviewing set of approvals
func (r *Runner) processRevision(ctx workflow.Context, prRevision receiver.Revision) {
	r.state = Working
	defer func() {
		r.lastProcessedRevision = prRevision.Revision
		r.state = Waiting
	}()
	failedPolicies := sync.Map{}
	var wg sync.WaitGroup
	wg.Add(len(prRevision.Roots))
	for _, root := range prRevision.Roots {
		rootCtx := workflow.WithValue(ctx, internalContext.ProjectKey, root.Name)
		workflow.Go(rootCtx, func(c workflow.Context) {
			defer wg.Done()
			failedRootPolicies, err := r.runTerraformWorkflow(c, root, prRevision)
			if err != nil {
				// choosing to not fail workflow and let it continue to exist
				// until PR close/timeout
				errCtx := workflow.WithValue(ctx, internalContext.ErrKey, err)
				workflow.GetLogger(errCtx).Error("processing pr revision")
			}
			// consolidate failures across all roots
			// policy sets are identical so multiple roots can fail the same policy without issue
			for _, failedPolicy := range failedRootPolicies {
				failedPolicies.LoadOrStore(failedPolicy.Name, failedPolicy)
			}
		})
	}
	wg.Wait()
	// TODO: check for policy failures
}

func (r *Runner) runTerraformWorkflow(ctx workflow.Context, root terraform.Root, prRevision receiver.Revision) ([]activities.PolicySet, error) {
	id, err := sideeffect.GenerateUUID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "generating uuid")
	}
	prRootInfo := internalTerraform.PRRootInfo{
		ID: id,
		Commit: github.Commit{
			Revision: prRevision.Revision,
		},
		Root: root,
		Repo: prRevision.Repo,
	}
	failedPolicies, err := r.TFWorkflowRunner.Run(ctx, prRootInfo)
	if err != nil {
		return nil, errors.Wrap(err, "running terraform workflow")
	}
	return failedPolicies, err
}

func (r *Runner) shouldProcessRevision(ctx workflow.Context, prRevision receiver.Revision) (bool, error) {
	if r.state == Working {
		return false, nil
	}
	if r.lastProcessedRevision == "" {
		return true, nil
	}
	direction, err := r.getCommitDirection(ctx, prRevision)
	if err != nil {
		return false, err
	}
	return direction != activities.DirectionBehind, nil
}

func (r *Runner) getCommitDirection(ctx workflow.Context, prRevision receiver.Revision) (activities.DiffDirection, error) {
	// this means we are deploying this root for the first time
	if r.lastProcessedRevision == "" {
		return activities.DirectionAhead, nil
	}
	var compareCommitResp activities.CompareCommitResponse
	err := workflow.ExecuteActivity(ctx, r.Activities.GithubCompareCommit, activities.CompareCommitRequest{
		DeployRequestRevision:  prRevision.Revision,
		LatestDeployedRevision: r.lastProcessedRevision,
		Repo:                   prRevision.Repo,
	}).Get(ctx, &compareCommitResp)
	if err != nil {
		return "", errors.Wrap(err, "unable to determine new revision commit direction")
	}
	return compareCommitResp.CommitComparison, nil
}
