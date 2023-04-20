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
	"sync"
)

type Action int64

const (
	OnNewPRRevision Action = iota
	OnShutdown
)

type Runner struct {
	RevisionSignalChannel workflow.ReceiveChannel
	RevisionReceiver      *receiver.RevisionReceiver
	ShutdownSignalChannel workflow.ReceiveChannel
	ShutdownReceiver      *receiver.ShutdownReceiver

	Scope            workflowMetrics.Scope
	TFWorkflowRunner *internalTerraform.WorkflowRunner
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
func (r Runner) Run(ctx workflow.Context) error {
	var action Action
	var prRevision receiver.Revision
	var err error

	//TODO: add approve signal, timeouts, poll variations of signals
	s := workflow.NewSelector(ctx)
	s.AddReceive(r.RevisionSignalChannel, func(c workflow.ReceiveChannel, more bool) {
		prRevision = r.RevisionReceiver.Receive(c, more)
		action = OnNewPRRevision
	})

	s.AddReceive(r.ShutdownSignalChannel, func(c workflow.ReceiveChannel, more bool) {
		r.ShutdownReceiver.Receive(c, more)
		action = OnShutdown
	})

	inProgressCtx, inProgressCancel := workflow.WithCancel(ctx)
	for {
		if err != nil {
			return err
		}
		s.Select(ctx)
		switch action {
		case OnNewPRRevision:
			// cancel in progress workflow (if it exists) and start up a new one
			// TODO: compare revision
			inProgressCancel()
			inProgressCtx, inProgressCancel = workflow.WithCancel(ctx)
			inProgressCtx = workflow.WithValue(inProgressCtx, internalContext.SHAKey, prRevision.Revision)
			workflow.GetLogger(inProgressCtx).Info("received revision")
			workflow.Go(inProgressCtx, func(c workflow.Context) {
				err = r.processRevision(c, prRevision)
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
func (r Runner) processRevision(ctx workflow.Context, prRevision receiver.Revision) error {
	failedPolicies := sync.Map{}
	var wg sync.WaitGroup
	wg.Add(len(prRevision.Roots))
	for _, root := range prRevision.Roots {
		rootCtx := workflow.WithValue(ctx, internalContext.ProjectKey, root.Name)
		workflow.Go(rootCtx, func(c workflow.Context) {
			defer wg.Done()
			failedRootPolicies, err := r.runTerraformWorkflow(c, root, prRevision)
			if err != nil {
				errCtx := workflow.WithValue(ctx, internalContext.ErrKey, err)
				workflow.GetLogger(errCtx).Error("running terraform workflow")
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
	return nil
}

func (r Runner) runTerraformWorkflow(ctx workflow.Context, root terraform.Root, prRevision receiver.Revision) ([]activities.PolicySet, error) {
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
