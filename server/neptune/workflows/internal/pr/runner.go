package pr

import (
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
)

type Action int64

const (
	onNewRevision Action = iota
	onShutdown
)

type RunnerState int64

const (
	working RunnerState = iota
	waiting
	complete
)

type Runner struct {
	RevisionSignalChannel workflow.ReceiveChannel
	RevisionReceiver      *receiver.RevisionReceiver
	ShutdownSignalChannel workflow.ReceiveChannel
	ShutdownReceiver      *receiver.ShutdownReceiver
	Scope                 workflowMetrics.Scope
	TFWorkflowRunner      *internalTerraform.WorkflowRunner

	// mutable state
	state                 RunnerState
	lastAttemptedRevision string
}

func newRunner(ctx workflow.Context, scope workflowMetrics.Scope, tfWorkflowRunner *internalTerraform.WorkflowRunner) *Runner {
	revisionReceiver := receiver.NewRevisionReceiver(ctx, scope)
	shutdownReceiver := receiver.NewShutdownReceiver(ctx, scope)
	return &Runner{
		RevisionSignalChannel: workflow.GetSignalChannel(ctx, receiver.TerraformRevisionSignalID),
		RevisionReceiver:      &revisionReceiver,
		ShutdownSignalChannel: workflow.GetSignalChannel(ctx, receiver.ShutdownSignalID),
		ShutdownReceiver:      &shutdownReceiver,
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
		action = onNewRevision
	})

	s.AddReceive(r.ShutdownSignalChannel, func(c workflow.ReceiveChannel, more bool) {
		r.ShutdownReceiver.Receive(c, more)
		action = onShutdown
		r.state = complete
	})

	inProgressCtx, inProgressCancel := workflow.WithCancel(ctx)
	for {
		s.Select(ctx)
		switch action {
		case onNewRevision:
			ctx = workflow.WithValue(ctx, internalContext.SHAKey, prRevision.Revision)
			workflow.GetLogger(ctx).Info("received revision")
			if process := r.shouldProcessRevision(prRevision); !process {
				continue
			}
			// cancel in progress workflow (if it exists) and start up a new one
			inProgressCancel()
			inProgressCtx, inProgressCancel = workflow.WithCancel(ctx)
			r.state = working
			r.lastAttemptedRevision = prRevision.Revision
			workflow.Go(inProgressCtx, func(c workflow.Context) {
				r.processRevision(c, prRevision)
			})
		case onShutdown:
			//todo: maybe optimize by cancelling in progress child workflows
			workflow.GetLogger(ctx).Info("received shutdown request")
			return nil
		}
	}
}

// processRevision handles spinning off child Terraform workflows per root and
// dealing with any failed policies by reviewing set of approvals
func (r *Runner) processRevision(ctx workflow.Context, prRevision receiver.Revision) {
	defer func() {
		r.state = waiting
	}()
	failedPolicies := make(map[string]activities.PolicySet)
	remainingRoots := len(prRevision.Roots)
	for _, root := range prRevision.Roots {
		rootCtx := workflow.WithValue(ctx, internalContext.ProjectKey, root.Name)
		workflow.Go(rootCtx, func(c workflow.Context) {
			defer func() {
				remainingRoots--
			}()
			failedRootPolicies, err := r.runTerraformWorkflow(c, root, prRevision)
			if err != nil {
				// choosing to not fail workflow and let it continue to exist
				// until PR close/timeout
				workflow.GetLogger(workflow.WithValue(ctx, internalContext.ErrKey, err)).Error("processing pr revision")
			}
			// consolidate failures across all roots
			// policy sets are identical so multiple roots can fail the same policy without issue
			for _, failedPolicy := range failedRootPolicies {
				failedPolicies[failedPolicy.Name] = failedPolicy
			}
		})
	}
	err := workflow.Await(ctx, func() bool {
		return remainingRoots == 0
	})
	if err != nil {
		workflow.GetLogger(workflow.WithValue(ctx, internalContext.ErrKey, err)).Error("await error")
		return
	}
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

func (r *Runner) shouldProcessRevision(prRevision receiver.Revision) bool {
	// ignore reruns when revision is still in progress
	if r.lastAttemptedRevision == prRevision.Revision && r.state != waiting {
		return false
	}
	return true
}
