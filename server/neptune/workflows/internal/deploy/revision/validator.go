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

type githubActivities interface {
	CompareCommit(ctx context.Context, request activities.CompareCommitRequest) (activities.CompareCommitResponse, error)
}

// Validates the deploy request commit and updates the github Checkrun UI
type Validator struct {
	Activity githubActivities
}

func (v *Validator) IsRevisionValid(ctx workflow.Context, repo github.Repo, deployRequest terraform.DeploymentInfo, latestDeployment *root.DeploymentInfo) (bool, error) {
	if latestDeployment.Revision == deployRequest.Revision {
		return false, nil
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
		// TODO: Update checkrun [Deployed Revision is identical to the deploy request revision]
		logger.Info(ctx, fmt.Sprintf("Deployed Revision: %s is identical to the Deploy Request Revision: %s.", latestDeployment.Revision, deployRequest.Revision))
		return false, nil

	case activities.DirectionBehind:
		// TODO: Update checkrun [Deployed Revision is ahead of the deploy request revision]
		logger.Info(ctx, fmt.Sprintf("Deployed Revision: %s is ahead of the Deploy Request Revision: %s.", latestDeployment.Revision, deployRequest.Revision))
		return false, nil

	case activities.DirectionDiverged:
		// TODO: Check for Force Apply
		logger.Info(ctx, fmt.Sprintf("Deployed Revision: %s is divergent from the Deploy Request Revision: %s.", latestDeployment.Revision, deployRequest.Revision))
		return false, nil

	case activities.DirectionAhead:
		logger.Info(ctx, fmt.Sprintf("Deployed Revision: %s is divergent from the Deploy Request Revision: %s.", latestDeployment.Revision, deployRequest.Revision))
		return true, nil

	default:
		return false, fmt.Errorf("invalid commit comparison: %s", compareCommitResp.CommitComparison)
	}
}
