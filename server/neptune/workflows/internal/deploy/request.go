package deploy

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/steps"
)

// Types defined here should not be used internally, as our goal should be to eventually swap these out for something less brittle than json translation
type Request struct {
	GHRequestID string
	Repository  steps.Repo
	Root        steps.Root
}
