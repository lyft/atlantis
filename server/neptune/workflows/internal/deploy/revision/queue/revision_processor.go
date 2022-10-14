package queue

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/config/logger"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/root"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type githubActivities interface {
	CompareCommit(ctx context.Context, request activities.CompareCommitRequest) (activities.CompareCommitResponse, error)
	UpdateCheckRun(ctx context.Context, request activities.UpdateCheckRunRequest) (activities.UpdateCheckRunResponse, error)
}

type RevisionProcessor struct {
	Activities githubActivities
}

const (
	DirectionBehindSummary   = "This revision is behind the current revision and will not be deployed.  If this is intentional, revert the default branch to this revision to trigger a new deployment."
	UpdateCheckRunRetryCount = 5

	ForceApplySummary = "The current deployment has diverged from the default branch, so we have locked the root. This is most likely the result of this PR performing a deployment. To override that lock and allow the main branch to perform new deployments, select the Unlock button."
)

func (p *RevisionProcessor) Process(ctx workflow.Context, requestedDeployment terraform.DeploymentInfo, latestDeployment *root.DeploymentInfo) error {
	commitDirection, err := p.getDeployRequestCommitDirection(ctx, requestedDeployment, latestDeployment)
	if err != nil {
		return err
	}

	logger.Info(ctx, fmt.Sprintf("relationship of deployed to requested revision: %s", commitDirection),
		"deployed-revision", latestDeployment.GetRevision(),
		"requested-revision", requestedDeployment.Revision)

	switch commitDirection {
	case activities.DirectionBehind:
		p.updateCheckRun(ctx, requestedDeployment, github.CheckRunFailure, DirectionBehindSummary)
		return errors.New("requested revision is behind current one")
	case activities.DirectionDiverged:
		return p.lock(ctx, requestedDeployment)
	}
	return nil
}

func (p *RevisionProcessor) getDeployRequestCommitDirection(ctx workflow.Context, deployRequest terraform.DeploymentInfo, latestDeployment *root.DeploymentInfo) (activities.DiffDirection, error) {
	// root being deployed for the first time
	if latestDeployment == nil {
		return activities.DirectionAhead, nil
	}

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
func (p *RevisionProcessor) updateCheckRun(ctx workflow.Context, deployRequest terraform.DeploymentInfo, state github.CheckRunState, summary string) {
	ctx = workflow.WithRetryPolicy(ctx, temporal.RetryPolicy{
		MaximumAttempts: UpdateCheckRunRetryCount,
	})

	err := workflow.ExecuteActivity(ctx, p.Activities.UpdateCheckRun, activities.UpdateCheckRunRequest{
		Title:   terraform.BuildCheckRunTitle(deployRequest.Root.Name),
		State:   state,
		Repo:    deployRequest.Repo,
		ID:      deployRequest.CheckRunID,
		Summary: summary,
	}).Get(ctx, nil)
	if err != nil {
		logger.Error(ctx, "unable to update checkrun", err.Error())
	}
}

// For merged deployments, notify user of a force apply lock status and lock future deployments until signal is received
func (p *RevisionProcessor) lock(ctx workflow.Context, deploymentInfo terraform.DeploymentInfo) error {
	// We won't lock any manually triggered
	if deploymentInfo.Root.Trigger == root.ManualTrigger {
		return nil
	}
	request := activities.UpdateCheckRunRequest{
		Title:   terraform.BuildCheckRunTitle(deploymentInfo.Root.Name),
		State:   github.CheckRunPending,
		Repo:    deploymentInfo.Repo,
		ID:      deploymentInfo.CheckRunID,
		Summary: ForceApplySummary,
		Actions: []github.CheckRunAction{github.CreateUnlockAction()},
	}
	var resp activities.UpdateCheckRunResponse
	err := workflow.ExecuteActivity(ctx, p.Activities.UpdateCheckRun, request).Get(ctx, &resp)
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
