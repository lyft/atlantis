package converter

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/execute"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/request"
)

func Root(external request.Root) terraform.Root {
	return terraform.Root{
		Name: external.Name,
		Plan: terraform.PlanJob{
			Job: execute.Job{
				Steps: steps(external.Plan.Steps)},
			Mode: mode(external.PlanMode),
		},
		Validate: execute.Job{
			Steps: steps(external.Validate.Steps),
		},
		Path:      external.RepoRelPath,
		TfVersion: external.TfVersion,
	}
}

func mode(mode request.PlanMode) *terraform.PlanMode {
	switch mode {
	case request.DestroyPlanMode:
		return terraform.NewDestroyPlanMode()
	}

	return nil
}

func steps(requestSteps []request.Step) []execute.Step {
	var terraformSteps []execute.Step
	for _, step := range requestSteps {
		terraformSteps = append(terraformSteps, execute.Step{
			StepName:    step.StepName,
			ExtraArgs:   step.ExtraArgs,
			RunCommand:  step.RunCommand,
			EnvVarName:  step.EnvVarName,
			EnvVarValue: step.EnvVarValue,
		})
	}
	return terraformSteps
}
