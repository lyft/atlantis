package queue

import (
	"testing"

	"github.com/google/uuid"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/terraform"
	"github.com/stretchr/testify/assert"
)

func TestWorker_IsDeploying(t *testing.T) {
	d := terraform.DeploymentInfo{ID: uuid.New()}

	// InProgressStatus is the zero value, so a worker that hasn't deployed
	// anything must not count as deploying.
	assert.False(t, (&Worker{}).IsDeploying())

	w := &Worker{}
	w.setCurrentDeploymentState(CurrentDeployment{Deployment: d, Status: InProgressStatus})
	assert.True(t, w.IsDeploying())

	w.setCurrentDeploymentState(CurrentDeployment{Deployment: d, Status: CompleteStatus})
	assert.False(t, w.IsDeploying())
}
