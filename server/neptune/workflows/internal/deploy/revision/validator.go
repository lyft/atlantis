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

type Validator struct {
	Activity githubActivities
}

func (v *Validator) IsRevisionValid(ctx workflow.Context, repo github.Repo, deployedRequestRevision terraform.DeploymentInfo, latestDeployedRevision *root.DeploymentInfo) (bool, error) {
	aheadBy, err := v.deployRequestRevisionIsAheadBy(ctx, repo, deployedRequestRevision, latestDeployedRevision)
	if err != nil {
		// TODO: Update the checkrun
		return false, errors.Wrap(err, "validating deploy request revision")
	}

	switch {
	// TODO: Update checkrun [Deployed Revision is identical to the deploy request revision]
	case aheadBy == 0:
		logger.Info(ctx, fmt.Sprintf("Deployed Revision: %s is identical to the Deploy Request Revision: %s.", latestDeployedRevision.Revision, deployedRequestRevision.Revision))
		return false, nil

	// TODO: Update checkrun [Deployed Revision is ahead of the deploy request revision]
	case aheadBy < 0:
		logger.Info(ctx, fmt.Sprintf("Deployed Revision: %s is ahead of the Deploy Request Revision: %s.", latestDeployedRevision.Revision, deployedRequestRevision.Revision))
		return false, nil
	}

	return true, nil
}

func (w *Validator) deployRequestRevisionIsAheadBy(ctx workflow.Context, repo github.Repo, deployeRequestRevision terraform.DeploymentInfo, latestDeployedRevision *root.DeploymentInfo) (int, error) {
	// skip compare commit if deploy request revision is the same as latest deployed revision
	if latestDeployedRevision.Revision == deployeRequestRevision.Revision {
		return 0, nil
	}

	var compareCommitResp activities.CompareCommitResponse
	err := workflow.ExecuteActivity(ctx, w.Activity.CompareCommit, activities.CompareCommitRequest{
		DeployRequestRevision:  deployeRequestRevision.Revision,
		LatestDeployedRevision: latestDeployedRevision.Revision,
		Repo:                   repo,
	}).Get(ctx, &compareCommitResp)
	if err != nil {
		return 0, errors.Wrap(err, "comparing revision")
	}

	return compareCommitResp.DeployRequestRevisionAheadBy, nil
}
