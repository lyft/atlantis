package terraform

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
)

type Response struct {
	ValidationResults []activities.ValidationResult
	WorkflowState     state.Workflow
}
