package event

import (
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
)

const defaultWorkspace = "default"

type StepGenerator interface {
	GeneratePlanSteps(repoID string) []workflows.Step
	GenerateApplySteps(repoID string) []workflows.Step
}

type TerraformWorkflowStepsGenerator struct {
	Logger    logging.Logger
	GlobalCfg valid.GlobalCfg
}

func (t *TerraformWorkflowStepsGenerator) GeneratePlanSteps(repoID string) []workflows.Step {
	// NOTE: for deployment workflows, we won't support command level user requests for log level output verbosity
	var workflowSteps []workflows.Step

	// TODO: replace example use of DefaultProjCfg with a project config generator that handles default vs. merged cfg case
	projectConfig := t.GlobalCfg.DefaultProjCfg(t.Logger, repoID, "path", defaultWorkspace)
	steps := projectConfig.Workflow.Plan.Steps
	for _, step := range steps {
		workflowSteps = append(workflowSteps, workflows.Step{
			StepName:    step.StepName,
			ExtraArgs:   step.ExtraArgs,
			RunCommand:  step.RunCommand,
			EnvVarName:  step.EnvVarName,
			EnvVarValue: step.EnvVarValue,
		})
	}
	return workflowSteps
}

func (t *TerraformWorkflowStepsGenerator) GenerateApplySteps(repoID string) []workflows.Step {
	var workflowSteps []workflows.Step
	projectConfig := t.GlobalCfg.DefaultProjCfg(t.Logger, repoID, "path", defaultWorkspace)
	steps := projectConfig.Workflow.Apply.Steps
	for _, step := range steps {
		workflowSteps = append(workflowSteps, workflows.Step{
			StepName:    step.StepName,
			ExtraArgs:   step.ExtraArgs,
			RunCommand:  step.RunCommand,
			EnvVarName:  step.EnvVarName,
			EnvVarValue: step.EnvVarValue,
		})
	}
	return workflowSteps
}
