package terraform_test

import (
	"testing"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/stretchr/testify/assert"
)

func TestSummary(t *testing.T) {
	plan := "{\"format_version\": \"1.0\",\"resource_changes\":[{\"change\":{\"actions\":[\"update\"]},\"address\":\"type.resource_update\"},{\"change\":{\"actions\":[\"create\"]},\"address\":\"type.resource_create\"}, {\"change\":{\"actions\":[\"delete\"]},\"address\":\"type.resource_delete\"}]}"

	t.Run("success", func(t *testing.T) {
		summary, err := terraform.NewPlanSummaryFromJSON([]byte(plan))
		assert.NoError(t, err)

		assert.Equal(t, terraform.PlanSummary{
			Creations: []terraform.ResourceSummary{
				{
					Address: "type.resource_create",
				},
			},
			Updates: []terraform.ResourceSummary{
				{
					Address: "type.resource_update",
				},
			},
			Deletions: []terraform.ResourceSummary{
				{
					Address: "type.resource_delete",
				},
			},
		}, summary)
	})

	t.Run("error", func(t *testing.T) {
		_, err := terraform.NewPlanSummaryFromJSON([]byte("{{"))
		assert.Error(t, err)
	})
}

func TestSummary_replace(t *testing.T) {
	plan := "{\"format_version\": \"1.0\",\"resource_changes\":[{\"change\":{\"actions\":[\"create\", \"delete\"]},\"address\":\"type.resource_replace\"}]}"
	summary, err := terraform.NewPlanSummaryFromJSON([]byte(plan))
	assert.NoError(t, err)

	assert.Equal(t, terraform.PlanSummary{
		Creations: []terraform.ResourceSummary{
			{
				Address: "type.resource_replace",
			},
		},
		Deletions: []terraform.ResourceSummary{
			{
				Address: "type.resource_replace",
			},
		},
	}, summary)
}

func TestSummary_empty(t *testing.T) {
	plan := "{\"format_version\": \"1.0\",\"resource_changes\":[{\"change\":{\"actions\":[\"noop\"]},\"address\":\"type.resource_replace\"}]}"

	summary, err := terraform.NewPlanSummaryFromJSON([]byte(plan))
	assert.NoError(t, err)

	assert.Equal(t, terraform.PlanSummary{}, summary)
	assert.True(t, summary.IsEmpty())
}

func TestSummary_string(t *testing.T) {
	plan := "{\"format_version\": \"1.0\",\"resource_changes\":[{\"change\":{\"actions\":[\"update\"]},\"address\":\"type.resource_update\"},{\"change\":{\"actions\":[\"create\"]},\"address\":\"type.resource_create\"}, {\"change\":{\"actions\":[\"delete\"]},\"address\":\"type.resource_delete\"}]}"

	summary, err := terraform.NewPlanSummaryFromJSON([]byte(plan))
	assert.NoError(t, err)

	assert.Equal(t, "Plan: 1 to add, 1 to change, 1 to destroy.", summary.String())
}

func TestSummary_import(t *testing.T) {
	plan := `{"format_version": "1.0","resource_changes":[{"change":{"actions":["no-op"],"importing":{"id":"i-123"}},"address":"type.resource_import"}]}`

	summary, err := terraform.NewPlanSummaryFromJSON([]byte(plan))
	assert.NoError(t, err)

	assert.Equal(t, []terraform.ResourceSummary{{Address: "type.resource_import"}}, summary.Imports)
	assert.Equal(t, "Plan: 1 to import, 0 to add, 0 to change, 0 to destroy.", summary.String())
	// The review gate auto-approves plans where IsEmpty is true, so import-only
	// plans keep their existing behavior.
	assert.True(t, summary.IsEmpty())
}

func TestSummary_forget(t *testing.T) {
	plan := `{"format_version": "1.0","resource_changes":[{"change":{"actions":["forget"]},"address":"type.resource_forget"}]}`

	summary, err := terraform.NewPlanSummaryFromJSON([]byte(plan))
	assert.NoError(t, err)

	assert.Equal(t, []terraform.ResourceSummary{{Address: "type.resource_forget"}}, summary.Forgets)
	assert.Equal(t, "Plan: 0 to add, 0 to change, 0 to destroy, 1 to forget.", summary.String())
	assert.True(t, summary.IsEmpty())
}

func TestSummary_importAndForget(t *testing.T) {
	plan := `{"format_version": "1.0","resource_changes":[` +
		`{"change":{"actions":["update"],"importing":{"id":"i-123"}},"address":"type.resource_import"},` +
		`{"change":{"actions":["forget"]},"address":"type.resource_forget"}]}`

	summary, err := terraform.NewPlanSummaryFromJSON([]byte(plan))
	assert.NoError(t, err)

	assert.Equal(t, "Plan: 1 to import, 0 to add, 1 to change, 0 to destroy, 1 to forget.", summary.String())
	assert.False(t, summary.IsEmpty())
}

func TestSummary_drift(t *testing.T) {
	plan := `{"format_version": "1.0",` +
		`"resource_drift":[{"change":{"actions":["update"]},"address":"type.drifted_a"},{"change":{"actions":["delete"]},"address":"type.drifted_b"}],` +
		`"resource_changes":[{"change":{"actions":["update"]},"address":"type.resource_update"}]}`

	summary, err := terraform.NewPlanSummaryFromJSON([]byte(plan))
	assert.NoError(t, err)

	assert.Equal(t, []terraform.ResourceSummary{{Address: "type.drifted_a"}, {Address: "type.drifted_b"}}, summary.Drift)
	assert.Equal(t, "Note: 2 resources have changed outside of Terraform.", summary.DriftNote())
	// drift is informational and never counted as a planned change
	assert.Equal(t, "Plan: 0 to add, 1 to change, 0 to destroy.", summary.String())
}

func TestSummary_noDrift(t *testing.T) {
	assert.Equal(t, "", terraform.PlanSummary{}.DriftNote())
	assert.Equal(t, "Note: 1 resource has changed outside of Terraform.",
		terraform.PlanSummary{Drift: []terraform.ResourceSummary{{Address: "type.a"}}}.DriftNote())
}
