package pr

import (
	"context"
	"fmt"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"go.temporal.io/sdk/client"
)

type signaler interface {
	SignalWithStartWorkflow(ctx context.Context, workflowID string, signalName string, signalArg interface{},
		options client.StartWorkflowOptions, workflow interface{}, workflowArgs ...interface{}) (client.WorkflowRun, error)
}

const (
	Deprecated = "deprecated"
	Destroy    = "-destroy"
)

type WorkflowSignaler struct {
	TemporalClient signaler
}

type Options struct {
	Number            int
	Revision          string
	Repo              models.Repo
	InstallationToken int64
	Branch            string
}

func (s *WorkflowSignaler) SignalWithStartWorkflow(ctx context.Context, rootCfgs []*valid.MergedProjectCfg, prOptions Options) (client.WorkflowRun, error) {
	options := client.StartWorkflowOptions{
		TaskQueue: workflows.PRTaskQueue,
		SearchAttributes: map[string]interface{}{
			"atlantis_repository": prOptions.Repo.FullName,
		},
	}
	run, err := s.TemporalClient.SignalWithStartWorkflow(
		ctx,
		BuildPRWorkflowID(prOptions.Repo.FullName, prOptions.Number),
		workflows.PRTerraformRevisionSignalID,
		workflows.PRNewRevisionSignalRequest{
			Revision: prOptions.Revision,
			Roots:    buildRoots(rootCfgs),
			Repo: workflows.PRRepo{
				URL:           prOptions.Repo.CloneURL,
				FullName:      prOptions.Repo.FullName,
				Name:          prOptions.Repo.Name,
				Owner:         prOptions.Repo.Owner,
				DefaultBranch: prOptions.Repo.DefaultBranch,
				Credentials: workflows.PRAppCredentials{
					InstallationToken: prOptions.InstallationToken,
				},
			},
		},
		options,
		workflows.PR,
		workflows.PRRequest{
			RepoFullName: prOptions.Repo.FullName,
			PRNum:        prOptions.Number,
		},
	)
	return run, err
}

func BuildPRWorkflowID(repoName string, prNum int) string {
	return fmt.Sprintf("%s||%d", repoName, prNum)
}

func buildRoots(rootCfgs []*valid.MergedProjectCfg) []workflows.PRRoot {
	var roots []workflows.PRRoot
	for _, rootCfg := range rootCfgs {
		var tfVersion string
		if rootCfg.TerraformVersion != nil {
			tfVersion = rootCfg.TerraformVersion.String()
		}
		roots = append(roots, workflows.PRRoot{
			Name:        rootCfg.Name,
			RepoRelPath: rootCfg.RepoRelDir,
			TfVersion:   tfVersion,
			PlanMode:    generatePlanMode(rootCfg),
			Plan:        workflows.PRJob{Steps: generateSteps(rootCfg.PullRequestWorkflow.Plan.Steps)},
			Validate:    workflows.PRJob{Steps: generateSteps(rootCfg.PullRequestWorkflow.PolicyCheck.Steps)},
		})
	}
	return roots
}

func generateSteps(steps []valid.Step) []workflows.PRStep {
	// TODO: support command level user requests for log level output verbosity
	// this will involve appending an TF_LOG env kv pair
	// for comment events with a log level defined
	var workflowSteps []workflows.PRStep
	for _, step := range steps {
		workflowSteps = append(workflowSteps, workflows.PRStep{
			StepName:    step.StepName,
			ExtraArgs:   step.ExtraArgs,
			RunCommand:  step.RunCommand,
			EnvVarName:  step.EnvVarName,
			EnvVarValue: step.EnvVarValue,
		})
	}
	return workflowSteps
}

func generatePlanMode(cfg *valid.MergedProjectCfg) workflows.PRPlanMode {
	t, ok := cfg.Tags[Deprecated]
	if ok && t == Destroy {
		return workflows.PRDestroyPlanMode
	}
	return workflows.PRNormalPlanMode
}
