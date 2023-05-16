package terraform_test

import (
	"testing"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/deployment"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/stretchr/testify/assert"
)

func TestPlanAppr_Non_Default(t *testing.T) {
	output := terraform.BuildPlanApproval(terraform.DeploymentInfo{
		Repo:           github.Repo{Name: "nish", Owner: "owner", DefaultBranch: "main"},
		InitiatingUser: github.User{Username: "nishkrishnan"},
		Commit:         github.Commit{Branch: "test", Revision: "rev"},
	}, &deployment.Info{Branch: "main"}, activities.DirectionDiverged, metrics.NewNullableScope())

	assert.Equal(t, "Requested Revision has diverged from deployed revision `[rev](https://github.com/owner/nish/commit/rev)` triggered by @nishkrishnan\n\n:point_right: Please rebase onto the default branch to pull in the latest changes.\n\n", output.Reason)
}

func TestPlanAppr_Default(t *testing.T) {
	output := terraform.BuildPlanApproval(terraform.DeploymentInfo{
		Repo:           github.Repo{Name: "nish", Owner: "owner", DefaultBranch: "main"},
		InitiatingUser: github.User{Username: "nishkrishnan"},
		Commit:         github.Commit{Branch: "main", Revision: "rev"},
	}, &deployment.Info{Branch: "main"}, activities.DirectionDiverged, metrics.NewNullableScope())

	assert.Equal(t, "Requested Revision has diverged from deployed revision `[rev](https://github.com/owner/nish/commit/rev)` triggered by @nishkrishnan\n\nDeployed revision contains unmerged changes.  Deploying this revision could cause an outage, please confirm with revision owner @nishkrishnan whether this is desirable.\n\n", output.Reason)
}
