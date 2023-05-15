package terraform

import (
	constants "github.com/runatlantis/atlantis/server/events/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/deployment"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github/markdown"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
)

func BuildPlanApproval(requestedDeployment DeploymentInfo, latestDeployment *deployment.Info, diffDirection activities.DiffDirection, scope metrics.Scope) terraform.PlanApproval {
	if diffDirection == activities.DirectionDiverged {
		scope.SubScopeWithTags(map[string]string{
			constants.ManualOverrideReasonTag: DivergedMetric,
		}).Counter(constants.ManualOverride).Inc(1)

		rendered := markdown.RenderPlanConfirm(
			requestedDeployment.InitiatingUser.Username, 
			requestedDeployment.Commit,
			latestDeployment.Branch,
			requestedDeployment.Repo)

		return terraform.PlanApproval{
			Type:   terraform.ManualApproval,
			Reason: rendered,
		}
	}

	if requestedDeployment.Root.Trigger == terraform.ManualTrigger {
		return terraform.PlanApproval{
			Type:   terraform.ManualApproval,
			Reason: "Manually Triggered Deploys must be confirmed before proceeding.",
		}
	}

	return terraform.PlanApproval{}
}