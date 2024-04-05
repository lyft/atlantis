package adhoc

import (
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/neptune/adhoc/request"
	"github.com/runatlantis/atlantis/server/neptune/gateway/deploy"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/execute"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
)

type Repo = request.Repo
type Root = request.Root
type Job = request.Job
type Step = request.Step
type AppCredentials = request.AppCredentials
type PlanMode = request.PlanMode

func getRootsFromMergedProjectCfgs(rootCfgs []*valid.MergedProjectCfg) ([]terraform.Root, error) {
	roots := make([]terraform.Root, 0, len(rootCfgs))
	for _, rootCfg := range rootCfgs {

		root := convertMergedProjectCfgToRoot(rootCfg)
		terraformRoot := convertToTerraformRoot(root)
		roots = append(roots, terraformRoot)
	}
	return roots, nil
}

func convertMergedProjectCfgToRoot(rootCfg *valid.MergedProjectCfg) request.Root {
	var tfVersion string
	if rootCfg.TerraformVersion != nil {
		tfVersion = rootCfg.TerraformVersion.String()
	}

	return request.Root{
		Name: rootCfg.Name,
		Plan: request.Job{
			Steps: prependPlanEnvSteps(rootCfg),
		},
		Apply: request.Job{
			Steps: generateSteps(rootCfg.DeploymentWorkflow.Apply.Steps),
		},
		RepoRelPath:  rootCfg.RepoRelDir,
		TrackedFiles: rootCfg.WhenModified,
		TfVersion:    tfVersion,
		PlanMode:     generatePlanMode(rootCfg),
		// note we don't set TriggerInfo
	}
}

func convertToTerraformRoot(root Root) terraform.Root {
	return terraform.Root{
		Name: root.Name,
		Apply: execute.Job{
			Steps: steps(root.Apply.Steps),
		},
		Plan: terraform.PlanJob{
			Job: execute.Job{
				Steps: steps(root.Plan.Steps)},
		},
		// Note we don't have mode nor PlanApproval
		Path:      root.RepoRelPath,
		TfVersion: root.TfVersion,
	}
}

// These are copied here so that we don't have to use a workflowsignaler
func prependPlanEnvSteps(cfg *valid.MergedProjectCfg) []Step {
	var steps []Step
	if t, ok := cfg.Tags[deploy.Manifest]; ok {
		//this is a Lyft specific env var
		steps = append(steps, Step{
			StepName:    deploy.EnvStep,
			EnvVarName:  "MANIFEST_FILEPATH",
			EnvVarValue: t,
		})
	}
	steps = append(steps, generateSteps(cfg.DeploymentWorkflow.Plan.Steps)...)
	return steps
}

func generateSteps(steps []valid.Step) []Step {
	var workflowSteps []Step
	for _, step := range steps {
		workflowSteps = append(workflowSteps, Step{
			StepName:    step.StepName,
			ExtraArgs:   step.ExtraArgs,
			RunCommand:  step.RunCommand,
			EnvVarName:  step.EnvVarName,
			EnvVarValue: step.EnvVarValue,
		})
	}
	return workflowSteps
}

func generatePlanMode(cfg *valid.MergedProjectCfg) request.PlanMode {
	t, ok := cfg.Tags[deploy.Deprecated]
	if ok && t == deploy.Destroy {
		return request.DestroyPlanMode
	}

	return request.NormalPlanMode
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
