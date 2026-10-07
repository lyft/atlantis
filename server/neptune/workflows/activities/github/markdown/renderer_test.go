package markdown_test

import (
	"net/url"
	"testing"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github/markdown"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"github.com/stretchr/testify/assert"
)

func prPlanState(summary terraform.PlanSummary) *state.Workflow {
	mode := terraform.PR
	logURL, _ := url.Parse("https://atlantis.example.com/jobs/1234")
	return &state.Workflow{
		Mode: &mode,
		Plan: &state.Job{
			Status: state.SuccessJobStatus,
			Output: &state.JobOutput{URL: logURL, PlanSummary: summary},
		},
	}
}

func TestRenderWorkflowStateTmpl_planDriftNote(t *testing.T) {
	out := markdown.RenderWorkflowStateTmpl(prPlanState(terraform.PlanSummary{
		Updates: []terraform.ResourceSummary{{Address: "type.resource_update"}},
		Drift:   []terraform.ResourceSummary{{Address: "type.drifted"}},
	}))

	assert.Contains(t, out, "**Note: 1 resource has changed outside of Terraform.**")
	assert.Contains(t, out, "`Plan: 0 to add, 1 to change, 0 to destroy.`")
}

func TestRenderWorkflowStateTmpl_noDrift(t *testing.T) {
	out := markdown.RenderWorkflowStateTmpl(prPlanState(terraform.PlanSummary{
		Updates: []terraform.ResourceSummary{{Address: "type.resource_update"}},
	}))

	assert.NotContains(t, out, "outside of Terraform")
	assert.Contains(t, out, "`Plan: 0 to add, 1 to change, 0 to destroy.`")
}
