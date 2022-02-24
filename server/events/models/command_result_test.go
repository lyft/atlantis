package models_test

import (
	"errors"
	"testing"

	"github.com/runatlantis/atlantis/server/events/models"
	. "github.com/runatlantis/atlantis/testing"
)

func TestCommandResult_HasErrors(t *testing.T) {
	cases := map[string]struct {
		cr  models.CommandResult
		exp bool
	}{
		"error": {
			cr: models.CommandResult{
				Error: errors.New("err"),
			},
			exp: true,
		},
		"failure": {
			cr: models.CommandResult{
				Failure: "failure",
			},
			exp: true,
		},
		"empty results list": {
			cr: models.CommandResult{
				ProjectResults: []models.ProjectResult{},
			},
			exp: false,
		},
		"successful plan": {
			cr: models.CommandResult{
				ProjectResults: []models.ProjectResult{
					{
						PlanSuccess: &models.PlanSuccess{},
					},
				},
			},
			exp: false,
		},
		"successful apply": {
			cr: models.CommandResult{
				ProjectResults: []models.ProjectResult{
					{
						ApplySuccess: "success",
					},
				},
			},
			exp: false,
		},
		"single errored project": {
			cr: models.CommandResult{
				ProjectResults: []models.ProjectResult{
					{
						Error: errors.New("err"),
					},
				},
			},
			exp: true,
		},
		"single failed project": {
			cr: models.CommandResult{
				ProjectResults: []models.ProjectResult{
					{
						Failure: "failure",
					},
				},
			},
			exp: true,
		},
		"two successful projects": {
			cr: models.CommandResult{
				ProjectResults: []models.ProjectResult{
					{
						PlanSuccess: &models.PlanSuccess{},
					},
					{
						ApplySuccess: "success",
					},
				},
			},
			exp: false,
		},
		"one successful, one failed project": {
			cr: models.CommandResult{
				ProjectResults: []models.ProjectResult{
					{
						PlanSuccess: &models.PlanSuccess{},
					},
					{
						Failure: "failed",
					},
				},
			},
			exp: true,
		},
	}

	for descrip, c := range cases {
		t.Run(descrip, func(t *testing.T) {
			Equals(t, c.exp, c.cr.HasErrors())
		})
	}
}
