package terraform

import (
	"fmt"
	"strings"

	constants "github.com/runatlantis/atlantis/server/events/metrics"
	"github.com/runatlantis/atlantis/server/neptune/template"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/deployment"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
)

type PlanApprovalOverrideBuilder struct {
	template.Loader[]
}

func Build(requestedDeployment DeploymentInfo, latestDeployment *deployment.Info, diffDirection activities.DiffDirection, scope metrics.Scope) terraform.PlanApproval {
	var approvalType terraform.PlanApprovalType
	var reasons []string

	if diffDirection == activities.DirectionDiverged || requestedDeployment.Root.Trigger == terraform.ManualTrigger {
		approvalType = terraform.ManualApproval
	}

	if diffDirection == activities.DirectionDiverged {
		scope.SubScopeWithTags(map[string]string{
			constants.ManualOverrideReasonTag: DivergedMetric,
		}).Counter(constants.ManualOverride).Inc(1)

		reason := fmt.Sprintf("Requested Revision has diverged from deployed revision %s () triggered by %s")

		reasons = append(reasons, reason)
	}

	if requestedDeployment.Root.Trigger == terraform.ManualTrigger {
		reasons = append(reasons, "Manually Triggered Deploys must be confirmed before proceeding.")
	}

	return terraform.PlanApproval{
		Type:   approvalType,
		Reason: strings.Join(reasons, "\n"),
	}
}