package deploy

import (
	"context"
	"fmt"

	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"go.temporal.io/sdk/client"
)

type signaler interface {
	SignalWithStartWorkflow(ctx context.Context, workflowID string, signalName string, signalArg interface{},
		options client.StartWorkflowOptions, workflow interface{}, workflowArgs ...interface{}) (client.WorkflowRun, error)
	SignalWorkflow(ctx context.Context, workflowID string, runID string, signalName string, arg interface{}) error
}

const (
	Deprecated = "deprecated"
	Destroy    = "-destroy"
)

type WorkflowSignaler struct {
	TemporalClient signaler
}

func (d *WorkflowSignaler) SignalWithStartWorkflow(ctx context.Context, rootCfg *valid.MergedProjectCfg, rootDeployOptions RootDeployOptions) (client.WorkflowRun, error) {
	options := client.StartWorkflowOptions{
		TaskQueue: workflows.DeployTaskQueue,
		SearchAttributes: map[string]interface{}{
			"atlantis_repository": rootDeployOptions.Repo.FullName,
			"atlantis_root":       rootCfg.Name,
		},
	}

	repo := rootDeployOptions.Repo
	var tfVersion string
	if rootCfg.TerraformVersion != nil {
		tfVersion = rootCfg.TerraformVersion.String()
	}

	run, err := d.TemporalClient.SignalWithStartWorkflow(
		ctx,
		BuildDeployWorkflowID(repo.FullName, rootCfg.Name),
		workflows.DeployNewRevisionSignalID,
		workflows.DeployNewRevisionSignalRequest{
			Revision: rootDeployOptions.Revision,
			Branch:   rootDeployOptions.Branch,
			InitiatingUser: workflows.User{
				Name: rootDeployOptions.Sender.Username,
			},
			Root: workflows.Root{
				Name: rootCfg.Name,
				Plan: workflows.Job{
					Steps: d.generateSteps(rootCfg.DeploymentWorkflow.Plan.Steps),
				},
				Apply: workflows.Job{
					Steps: d.generateSteps(rootCfg.DeploymentWorkflow.Apply.Steps),
				},
				RepoRelPath:  rootCfg.RepoRelDir,
				TrackedFiles: rootCfg.WhenModified,
				TfVersion:    tfVersion,
				PlanMode:     d.generatePlanMode(rootCfg),
				TriggerInfo:  rootDeployOptions.TriggerInfo,
			},
			Repo: workflows.Repo{
				URL:      repo.CloneURL,
				FullName: repo.FullName,
				Name:     repo.Name,
				Owner:    repo.Owner,
				Credentials: workflows.AppCredentials{
					InstallationToken: rootDeployOptions.InstallationToken,
				},
				RebaseEnabled: true,
				DefaultBranch: repo.DefaultBranch,
			},
			Tags: rootCfg.Tags,
		},
		options,
		workflows.Deploy,
		workflows.DeployRequest{
			Repo: workflows.DeployRequestRepo{
				FullName: repo.FullName,
			},
			Root: workflows.DeployRequestRoot{
				Name: rootCfg.Name,
			},
		},
	)
	return run, err
}

func BuildDeployWorkflowID(repoName string, rootName string) string {
	return fmt.Sprintf("%s||%s", repoName, rootName)
}

func (d *WorkflowSignaler) generateSteps(steps []valid.Step) []workflows.Step {
	// NOTE: for deployment workflows, we won't support command level user requests for log level output verbosity
	var workflowSteps []workflows.Step
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

func (d *WorkflowSignaler) generatePlanMode(cfg *valid.MergedProjectCfg) workflows.PlanMode {
	t, ok := cfg.Tags[Deprecated]
	if ok && t == Destroy {
		return workflows.DestroyPlanMode
	}

	return workflows.NormalPlanMode
}

func (d *WorkflowSignaler) SignalWorkflow(ctx context.Context, workflowID string, runID string, signalName string, args interface{}) error {
	return d.TemporalClient.SignalWorkflow(ctx, workflowID, runID, signalName, args)
}
