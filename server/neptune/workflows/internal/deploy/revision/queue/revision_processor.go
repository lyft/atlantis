package queue

import (
	"context"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities/deployment"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/root"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type terraformWorkflowRunner interface {
	Run(ctx workflow.Context, deploymentInfo terraform.DeploymentInfo) error
}

type dbActivities interface {
	FetchLatestDeployment(ctx context.Context, request activities.FetchLatestDeploymentRequest) (activities.FetchLatestDeploymentResponse, error)
	StoreLatestDeployment(ctx context.Context, request activities.StoreLatestDeploymentRequest) error
}

type githubActivities interface {
	CompareCommit(ctx context.Context, request activities.CompareCommitRequest) (activities.CompareCommitResponse, error)
	UpdateCheckRun(ctx context.Context, request activities.UpdateCheckRunRequest) (activities.UpdateCheckRunResponse, error)
}

type workerActivities interface {
	dbActivities
	githubActivities
}

type RevisionProcessor struct {
	Activities              workerActivities
	TerraformWorkflowRunner terraformWorkflowRunner
}

const (
	DirectionBehindSummary   = "This revision is behind the current revision and will not be deployed.  If this is intentional, revert the default branch to this revision to trigger a new deployment."
	UpdateCheckRunRetryCount = 5

	DivergedCommitsSummary = "The current deployment has diverged from the default branch, so we have locked the root. This is most likely the result of this PR performing a manual deployment. To override that lock and allow the main branch to perform new deployments, select the Unlock button."
)

func (p *RevisionProcessor) Process(ctx workflow.Context, requestedDeployment terraform.DeploymentInfo, latestDeployment *deployment.Info) (*deployment.Info, error) {
	latestDeployment, err := p.fetchLatestDeployment(ctx, requestedDeployment, latestDeployment)
	if err != nil {
		return nil, err
	}
	commitDirection := activities.DirectionAhead
	// only fetch if last deployment exists
	if latestDeployment != nil {
		commitDirection, err = p.getDeployRequestCommitDirection(ctx, requestedDeployment, latestDeployment)
		if err != nil {
			return nil, err
		}
	}
	switch commitDirection {
	case activities.DirectionBehind:
		// always returns error for caller to skip revision
		if err = p.updateCheckRun(ctx, requestedDeployment, github.CheckRunFailure, DirectionBehindSummary, nil); err != nil {
			return nil, errors.Wrap(err, "updating check run")
		}
	case activities.DirectionDiverged:
		if err = p.waitForUserUnlock(ctx, requestedDeployment); err != nil {
			return nil, errors.Wrap(err, "waiting for user unlock")
		}
	}
	err = p.TerraformWorkflowRunner.Run(ctx, requestedDeployment)
	if err != nil {
		return nil, errors.Wrap(err, "running terraform workflow")
	}
	latestDeployment = p.buildLatestDeployment(requestedDeployment)

	// TODO: Persist deployment on shutdown if it fails instead of blocking
	if err = p.persistLatestDeployment(ctx, latestDeployment); err != nil {
		return nil, errors.Wrap(err, "failed to persist latest deploy job")
	}
	return latestDeployment, nil
}

func (p *RevisionProcessor) getDeployRequestCommitDirection(ctx workflow.Context, deployRequest terraform.DeploymentInfo, latestDeployment *deployment.Info) (activities.DiffDirection, error) {
	var compareCommitResp activities.CompareCommitResponse
	err := workflow.ExecuteActivity(ctx, p.Activities.CompareCommit, activities.CompareCommitRequest{
		DeployRequestRevision:  deployRequest.Revision,
		LatestDeployedRevision: latestDeployment.Revision,
		Repo:                   deployRequest.Repo,
	}).Get(ctx, &compareCommitResp)
	if err != nil {
		return "", errors.Wrap(err, "unable to determine deploy request commit direction")
	}
	return compareCommitResp.CommitComparison, nil
}

// worker should not block on updating check runs for invalid deploy requests so let's retry for UpdateCheckrunRetryCount only
func (p *RevisionProcessor) updateCheckRun(ctx workflow.Context, deployRequest terraform.DeploymentInfo, state github.CheckRunState, summary string, actions []github.CheckRunAction) error {
	ctx = workflow.WithRetryPolicy(ctx, temporal.RetryPolicy{
		MaximumAttempts: UpdateCheckRunRetryCount,
	})
	return workflow.ExecuteActivity(ctx, p.Activities.UpdateCheckRun, activities.UpdateCheckRunRequest{
		Title:   terraform.BuildCheckRunTitle(deployRequest.Root.Name),
		State:   state,
		Repo:    deployRequest.Repo,
		ID:      deployRequest.CheckRunID,
		Summary: summary,
		Actions: actions,
	}).Get(ctx, nil)
}

// For merged deployments, notify user of a force apply lock status and lock future deployments until signal is received
func (p *RevisionProcessor) waitForUserUnlock(ctx workflow.Context, deploymentInfo terraform.DeploymentInfo) error {
	// We won't lock a manually triggered root
	if deploymentInfo.Root.Trigger == root.ManualTrigger {
		return nil
	}
	err := p.updateCheckRun(ctx, deploymentInfo, github.CheckRunPending, DivergedCommitsSummary, []github.CheckRunAction{github.CreateUnlockAction()})
	if err != nil {
		return errors.Wrap(err, "updating check run")
	}
	// Wait for unlock signal
	signalChan := workflow.GetSignalChannel(ctx, UnlockSignalName)
	var unlockRequest UnlockSignalRequest
	_ = signalChan.Receive(ctx, &unlockRequest)
	// TODO: store info on user that unlocked revision, maybe within the check run or just log it?
	return nil
}

func (p *RevisionProcessor) fetchLatestDeployment(ctx workflow.Context, deploymentInfo terraform.DeploymentInfo, latestDeployment *deployment.Info) (*deployment.Info, error) {
	// Skip fetching latest deployment it it's already in memory
	if latestDeployment != nil {
		return latestDeployment, nil
	}
	var resp activities.FetchLatestDeploymentResponse
	err := workflow.ExecuteActivity(ctx, p.Activities.FetchLatestDeployment, activities.FetchLatestDeploymentRequest{
		FullRepositoryName: deploymentInfo.Repo.GetFullName(),
		RootName:           deploymentInfo.Root.Name,
	}).Get(ctx, &resp)
	if err != nil {
		return nil, errors.Wrap(err, "fetching latest deployment")
	}
	return resp.DeploymentInfo, nil
}

func (p *RevisionProcessor) buildLatestDeployment(deployRequest terraform.DeploymentInfo) *deployment.Info {
	return &deployment.Info{
		Version:    DeploymentInfoVersion,
		ID:         deployRequest.ID.String(),
		CheckRunID: deployRequest.CheckRunID,
		Revision:   deployRequest.Revision,
		Root:       deployRequest.Root,
		Repo:       deployRequest.Repo,
	}
}

func (p *RevisionProcessor) persistLatestDeployment(ctx workflow.Context, deploymentInfo *deployment.Info) error {
	err := workflow.ExecuteActivity(ctx, p.Activities.StoreLatestDeployment, activities.StoreLatestDeploymentRequest{
		DeploymentInfo: deploymentInfo,
	}).Get(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "persisting deployment info")
	}
	return nil
}
