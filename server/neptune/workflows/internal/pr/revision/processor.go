package revision

import (
	"github.com/google/uuid"
	metricNames "github.com/runatlantis/atlantis/server/metrics"
	internalContext "github.com/runatlantis/atlantis/server/neptune/context"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	terraformActivities "github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/notifier"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/sideeffect"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	ReviewSignalID    = "pr-review"
	CheckRunCancelled = "checkrun was cancelled"
)

type TFWorkflow func(ctx workflow.Context, request terraform.Request) (terraform.Response, error)

type TFStateReceiver interface {
	Receive(ctx workflow.Context, c workflow.ReceiveChannel, rootCache map[string]RootInfo)
}

type PolicyHandler interface {
	Handle(ctx workflow.Context, prRevision Revision, roots map[string]RootInfo, workflowResponses []terraform.Response)
}

type CheckRunClient interface {
	CreateOrUpdate(ctx workflow.Context, id string, request notifier.GithubCheckRunRequest) (int64, error)
}

type Processor struct {
	TFStateReceiver     TFStateReceiver
	TFWorkflow          TFWorkflow
	PolicyHandler       PolicyHandler
	GithubCheckRunCache CheckRunClient
	Scope               metrics.Scope
}

// Process handles spinning off child Terraform workflows per root and
// dealing with any failed policies by reviewing set of approvals
func (p *Processor) Process(ctx workflow.Context, prRevision Revision) {
	roots := make(map[string]RootInfo)
	var futures []workflow.ChildWorkflowFuture
	scope := p.Scope.SubScope("policies").SubScopeWithTags(map[string]string{
		metricNames.RevisionTag: prRevision.Revision,
	})
	for _, root := range prRevision.Roots {
		id, err := sideeffect.GenerateUUID(ctx)
		if err != nil {
			workflow.GetLogger(workflow.WithValue(ctx, internalContext.ErrKey, err)).Error("generating uuid")
			continue
		}
		roots[id.String()] = RootInfo{
			ID: id,
			Commit: github.Commit{
				Revision: prRevision.Revision,
			},
			Root: root,
			Repo: prRevision.Repo,
		}
		future := p.processRoot(ctx, root, prRevision, id)
		futures = append(futures, future)
	}

	// Mark checkruns as aborted if the context was cancelled, this typically happens if revisions arrive in quick succession
	defer func() {
		if temporal.IsCanceledError(ctx.Err()) {
			ctx, _ := workflow.NewDisconnectedContext(ctx)
			p.markCheckRunsAborted(ctx, prRevision, roots)
			return
		}

		// At this point, all workflows should be successful, and we can mark combined plan check run as success
		p.markCombinedCheckRun(ctx, prRevision, github.CheckRunSuccess, "")
	}()

	terraformWorkflowResponses := p.awaitChildTerraformWorkflows(ctx, futures, roots)
	// Count all policy successes/failures + handle any failures by listening for approvals in PolicyHandler
	var failingTerraformWorkflowResponses []terraform.Response
	for _, resp := range terraformWorkflowResponses {
		for _, validationResult := range resp.ValidationResults {
			switch validationResult.Status {
			case activities.Fail:
				scope.SubScope(validationResult.PolicySet.Name).Counter(metricNames.ExecutionFailureMetric).Inc(1)
				failingTerraformWorkflowResponses = append(failingTerraformWorkflowResponses, resp)
			case activities.Success:
				scope.SubScope(validationResult.PolicySet.Name).Counter(metricNames.ExecutionSuccessMetric).Inc(1)
			}
		}
	}
	p.PolicyHandler.Handle(ctx, prRevision, roots, failingTerraformWorkflowResponses)
}

func (p *Processor) processRoot(ctx workflow.Context, root terraformActivities.Root, prRevision Revision, id uuid.UUID) workflow.ChildWorkflowFuture {
	ctx = workflow.WithValue(ctx, internalContext.ProjectKey, root.Name)
	ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: id.String(),
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
		// allows all signals to be received even in a cancellation state
		WaitForCancellation: true,
		SearchAttributes: map[string]interface{}{
			"atlantis_repository": prRevision.Repo.GetFullName(),
			"atlantis_root":       root.Name,
			"atlantis_trigger":    root.Trigger,
			"atlantis_revision":   prRevision.Revision,
		},
	})
	request := terraform.Request{
		Repo:         prRevision.Repo,
		Root:         root,
		DeploymentID: id.String(),
		Revision:     prRevision.Revision,
		WorkflowMode: terraformActivities.PR,
	}
	future := workflow.ExecuteChildWorkflow(ctx, p.TFWorkflow, request)
	return future
}

// awaitChildTerraformWorkflows creates a selector to listen to the completion of each root's child workflow future and any state
// change signals they send over the shared WorkflowStateChangeSignal channel; we only return when all workflows complete
func (p *Processor) awaitChildTerraformWorkflows(ctx workflow.Context, futures []workflow.ChildWorkflowFuture, roots map[string]RootInfo) []terraform.Response {
	selector := workflow.NewNamedSelector(ctx, "TerraformChildWorkflow")
	ch := workflow.GetSignalChannel(ctx, state.WorkflowStateChangeSignal)
	selector.AddReceive(ch, func(c workflow.ReceiveChannel, _ bool) {
		p.TFStateReceiver.Receive(ctx, c, roots)
	})

	var results []terraform.Response
	workflowsLeft := len(futures)
	for _, future := range futures {
		selector.AddFuture(future, func(f workflow.Future) {
			defer func() {
				workflowsLeft--
			}()
			var resp terraform.Response
			if err := f.Get(ctx, &resp); err != nil {
				// we will just log terraform workflow failures and continue trying to process other futures
				workflow.GetLogger(workflow.WithValue(ctx, internalContext.ErrKey, err)).Error("executing terraform workflow")
				return
			}
			results = append(results, resp)
		})
	}

	for {
		selector.Select(ctx)
		if workflowsLeft == 0 {
			break
		}
	}
	return results
}

func (p *Processor) markCombinedCheckRun(ctx workflow.Context, revision Revision, state github.CheckRunState, summary string) {
	ctx = workflow.WithRetryPolicy(ctx, temporal.RetryPolicy{
		MaximumAttempts: 3,
	})

	request := notifier.GithubCheckRunRequest{
		Title:   notifier.CombinedPlanCheckRunTitle,
		Sha:     revision.Revision,
		Repo:    revision.Repo,
		State:   state,
		Mode:    terraformActivities.PR,
		Summary: summary,
	}
	// ID is empty because we want to create a new check run
	_, err := p.GithubCheckRunCache.CreateOrUpdate(ctx, "", request)
	if err != nil {
		workflow.GetLogger(ctx).Error("unable to update check run with validation error", internalContext.ErrKey, err)
	}
}

func (p *Processor) markCheckRunsAborted(ctx workflow.Context, revision Revision, roots map[string]RootInfo) {
	p.markCombinedCheckRun(ctx, revision, github.CheckRunCancelled, CheckRunCancelled)

	for _, rootInfo := range roots {
		ctx = workflow.WithRetryPolicy(ctx, temporal.RetryPolicy{
			MaximumAttempts: 3,
		})

		request := notifier.GithubCheckRunRequest{
			Title:   notifier.BuildPlanCheckRunTitle(rootInfo.Root.Name),
			Sha:     revision.Revision,
			Repo:    revision.Repo,
			State:   github.CheckRunCancelled,
			Mode:    terraformActivities.PR,
			Summary: CheckRunCancelled,
		}
		_, err := p.GithubCheckRunCache.CreateOrUpdate(ctx, rootInfo.ID.String(), request)
		if err != nil {
			workflow.GetLogger(ctx).Error("unable to update check run with validation error", internalContext.ErrKey, err)
		}
	}
}
