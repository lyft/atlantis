package terraform

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
)

type Response struct {
	ValidationResults []activities.ValidationResult
}
