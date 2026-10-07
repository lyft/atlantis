package terraform

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-json"
	"github.com/pkg/errors"
)

// actionForget mirrors tfjson.ActionForget, which only exists in terraform-json
// v0.23.0+. Every release that has it also requires hashicorp/go-version >= 1.7.0,
// so it's defined locally rather than bumping a dependency used across the repo.
const actionForget tfjson.Action = "forget"

type ResourceSummary struct {
	Address string
}

type PlanSummary struct {
	Creations []ResourceSummary
	Deletions []ResourceSummary
	Updates   []ResourceSummary
	Imports   []ResourceSummary
	Forgets   []ResourceSummary
	// Drift holds resources that changed outside of Terraform since the last
	// apply. Drift is informational and is not a planned change.
	Drift []ResourceSummary
}

// IsEmpty reports whether the plan creates, updates, or deletes resources.
// The plan review gate auto-approves empty plans, so imports and forgets are
// deliberately not counted here.
func (s PlanSummary) IsEmpty() bool {
	return len(s.Creations) == 0 && len(s.Deletions) == 0 && len(s.Updates) == 0
}

func (s PlanSummary) hasPlannedChanges() bool {
	return !s.IsEmpty() || len(s.Imports) > 0 || len(s.Forgets) > 0
}

// Generates a super simple plan summary with changes grouped by action
// creation, deletion, update, import, and forget, plus any detected drift.
// changes are only represented using addresses for now.
func NewPlanSummaryFromJSON(b []byte) (PlanSummary, error) {
	if len(b) == 0 {
		return PlanSummary{}, nil
	}
	var plan tfjson.Plan
	err := json.Unmarshal(b, &plan)

	if err != nil {
		return PlanSummary{}, errors.Wrap(err, "parsing plan json")
	}

	var summary PlanSummary
	for _, c := range plan.ResourceChanges {
		resource := ResourceSummary{
			Address: c.Address,
		}
		actions := c.Change.Actions
		if actions.Delete() || actions.Replace() {
			summary.Deletions = append(summary.Deletions, resource)
		}

		if actions.Create() || actions.Replace() {
			summary.Creations = append(summary.Creations, resource)
		}

		if actions.Update() {
			summary.Updates = append(summary.Updates, resource)
		}

		if c.Change.Importing != nil {
			summary.Imports = append(summary.Imports, resource)
		}

		if len(actions) == 1 && actions[0] == actionForget {
			summary.Forgets = append(summary.Forgets, resource)
		}
	}

	for _, d := range plan.ResourceDrift {
		summary.Drift = append(summary.Drift, ResourceSummary{Address: d.Address})
	}

	return summary, nil
}

// String renders a one line summary in the same format as the Terraform CLI.
func (s PlanSummary) String() string {
	if !s.hasPlannedChanges() {
		return "No plan summary created. Most likely due to an error. Please check the logs."
	}
	add, change, destroy := len(s.Creations), len(s.Updates), len(s.Deletions)
	imports, forgets := len(s.Imports), len(s.Forgets)
	switch {
	case imports > 0 && forgets > 0:
		return fmt.Sprintf("Plan: %d to import, %d to add, %d to change, %d to destroy, %d to forget.", imports, add, change, destroy, forgets)
	case imports > 0:
		return fmt.Sprintf("Plan: %d to import, %d to add, %d to change, %d to destroy.", imports, add, change, destroy)
	case forgets > 0:
		return fmt.Sprintf("Plan: %d to add, %d to change, %d to destroy, %d to forget.", add, change, destroy, forgets)
	default:
		return fmt.Sprintf("Plan: %d to add, %d to change, %d to destroy.", add, change, destroy)
	}
}

// DriftNote returns a note about resources changed outside of Terraform, or an
// empty string when no drift was detected.
func (s PlanSummary) DriftNote() string {
	switch len(s.Drift) {
	case 0:
		return ""
	case 1:
		return "Note: 1 resource has changed outside of Terraform."
	default:
		return fmt.Sprintf("Note: %d resources have changed outside of Terraform.", len(s.Drift))
	}
}
