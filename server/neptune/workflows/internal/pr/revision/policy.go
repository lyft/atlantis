package revision

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"go.temporal.io/sdk/workflow"
)

type FailedPolicyHandler struct {
}

func (f *FailedPolicyHandler) Process(ctx workflow.Context, failedPolicies []activities.PolicySet) {
	//TODO implement me
	return
}
