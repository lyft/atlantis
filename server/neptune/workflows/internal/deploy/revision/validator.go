package revision

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/config/logger"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/root"
	"go.temporal.io/sdk/workflow"
)

const (
	IdenticalRevisonSummary = "This revision is identical to the current revision and will not be deployed"
	DirectionBehindSummary  = "This revision is behind the current revision and will not be deployed.  If this is intentional, revert the default branch to this revision to trigger a new deployment."
)

type githubActivities interface {
	CompareCommit(ctx context.Context, request activities.CompareCommitRequest) (activities.CompareCommitResponse, error)
	UpdateCheckRun(ctx context.Context, request activities.UpdateCheckRunRequest) (activities.UpdateCheckRunResponse, error)
}

// Validates the deploy request commit and updates the github Checkrun UI
type Validator struct {
	Activity githubActivities
}

func (v *Validator) IsValid(ctx workflow.Context, repo github.Repo, deployRequest terraform.DeploymentInfo, latestDeployment *root.DeploymentInfo) (bool, error) {
	if latestDeployment.Revision == deployRequest.Revision {
		logger.Info(ctx, fmt.Sprintf("Deployed Revision: %s is identical to the Deploy Request Revision: %s.", latestDeployment.Revision, deployRequest.Revision))
		err := v.updateCheckRun(ctx, deployRequest, repo, IdenticalRevisonSummary)
		return false, err
	}

	var compareCommitResp activities.CompareCommitResponse
	err := workflow.ExecuteActivity(ctx, v.Activity.CompareCommit, activities.CompareCommitRequest{
		DeployRequestRevision:  deployRequest.Revision,
		LatestDeployedRevision: latestDeployment.Revision,
		Repo:                   repo,
	}).Get(ctx, &compareCommitResp)
	if err != nil {
		return false, errors.Wrap(err, "comparing revision")
	}

	switch compareCommitResp.CommitComparison {
	case activities.DirectionIdentical:
		logger.Info(ctx, fmt.Sprintf("Deployed Revision: %s is identical to the Deploy Request Revision: %s.", latestDeployment.Revision, deployRequest.Revision))
		err := v.updateCheckRun(ctx, deployRequest, repo, IdenticalRevisonSummary)
		return false, err

	case activities.DirectionBehind:
		logger.Info(ctx, fmt.Sprintf("Deployed Revision: %s is ahead of the Deploy Request Revision: %s.", latestDeployment.Revision, deployRequest.Revision))
		err := v.updateCheckRun(ctx, deployRequest, repo, DirectionBehindSummary)
		return false, err

	// we should not see DirectionDiverged commits since we skip revision validation for Force Applies. So, let's log an error and return invalid revision
	case activities.DirectionDiverged:
		logger.Error(ctx, fmt.Sprintf("Deployed Revision: %s is divergent from the Deploy Request Revision: %s.", latestDeployment.Revision, deployRequest.Revision))
		return false, nil

	case activities.DirectionAhead:
		logger.Info(ctx, fmt.Sprintf("Deployed Revision: %s is ahead of the Deploy Request Revision: %s.", latestDeployment.Revision, deployRequest.Revision))
		return true, nil

	default:
		return false, fmt.Errorf("invalid commit comparison: %s", compareCommitResp.CommitComparison)
	}
}

func (v *Validator) updateCheckRun(ctx workflow.Context, deployRequest terraform.DeploymentInfo, repo github.Repo, summary string) error {
	return workflow.ExecuteActivity(ctx, v.Activity.UpdateCheckRun, activities.UpdateCheckRunRequest{
		Title:   terraform.BuildCheckRunTitle(deployRequest.Root.Name),
		State:   github.CheckRunSuccess,
		Repo:    repo,
		ID:      deployRequest.CheckRunID,
		Summary: summary,
	}).Get(ctx, nil)
}
