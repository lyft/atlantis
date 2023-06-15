package pr

import (
	"context"
	"fmt"
	"strconv"

	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/models"
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
	Manifest   = "manifest_path"
	EnvStep    = "env"
)

type ValidateEnvs struct {
	Workspace      string // TODO: remove when we deprecate legacy mode
	Username       string
	PullNum        int
	PullAuthor     string
	HeadCommit     string
	BaseBranchName string
}

type WorkflowSignaler struct {
	TemporalClient   signaler
	DefaultTFVersion string
}

type Request struct {
	Number            int
	Revision          string
	Repo              models.Repo
	InstallationToken int64
	Branch            string
	ValidateEnvs      []ValidateEnvs
}

func (s *WorkflowSignaler) SignalWithStartWorkflow(ctx context.Context, rootCfgs []*valid.MergedProjectCfg, request Request) (client.WorkflowRun, error) {
	options := client.StartWorkflowOptions{
		TaskQueue: workflows.PRTaskQueue,
		SearchAttributes: map[string]interface{}{
			"atlantis_repository": request.Repo.FullName,
		},
	}
	run, err := s.TemporalClient.SignalWithStartWorkflow(
		ctx,
		BuildPRWorkflowID(request.Repo.FullName, request.Number),
		workflows.PRTerraformRevisionSignalID,
		workflows.PRNewRevisionSignalRequest{
			Revision: request.Revision,
			Roots:    s.buildRoots(rootCfgs, request.ValidateEnvs...),
			Repo: workflows.PRRepo{
				URL:           request.Repo.CloneURL,
				FullName:      request.Repo.FullName,
				Name:          request.Repo.Name,
				Owner:         request.Repo.Owner,
				DefaultBranch: request.Repo.DefaultBranch,
				Credentials: workflows.PRAppCredentials{
					InstallationToken: request.InstallationToken,
				},
			},
		},
		options,
		workflows.PR,
		workflows.PRRequest{
			RepoFullName: request.Repo.FullName,
			PRNum:        request.Number,
			Organization: rootCfgs[0].PolicySets.Organization,
		},
	)
	return run, err
}

func BuildPRWorkflowID(repoName string, prNum int) string {
	return fmt.Sprintf("%s||%d", repoName, prNum)
}

func (s *WorkflowSignaler) buildRoots(rootCfgs []*valid.MergedProjectCfg, validateEnvOpts ...ValidateEnvs) []workflows.PRRoot {
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
			Validate:    workflows.PRJob{Steps: s.prependValidateEnvSteps(rootCfg, validateEnvOpts...)},
		})
	}
	return roots
}

func (s *WorkflowSignaler) prependValidateEnvSteps(rootCfg *valid.MergedProjectCfg, opts ...ValidateEnvs) []workflows.PRStep {
	for _, o := range opts {
		initialEnvSteps := s.generatePRModeEnvSteps(rootCfg, o)
		return append(initialEnvSteps, generateSteps(rootCfg.PullRequestWorkflow.PolicyCheck.Steps)...)
	}
	return generateSteps(rootCfg.PullRequestWorkflow.PolicyCheck.Steps)
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

func (s *WorkflowSignaler) generatePRModeEnvSteps(cfg *valid.MergedProjectCfg, validateEnvs ValidateEnvs) []workflows.PRStep {
	tfVersion := s.DefaultTFVersion
	if cfg.TerraformVersion != nil {
		tfVersion = cfg.TerraformVersion.String()
	}
	steps := []workflows.PRStep{
		{
			StepName:    EnvStep,
			EnvVarName:  "WORKSPACE",
			EnvVarValue: validateEnvs.Workspace,
		},
		{
			StepName:    EnvStep,
			EnvVarName:  "USER_NAME",
			EnvVarValue: validateEnvs.Username,
		},
		{
			StepName:    EnvStep,
			EnvVarName:  "PULL_NUM",
			EnvVarValue: strconv.Itoa(validateEnvs.PullNum),
		},
		{
			StepName:    EnvStep,
			EnvVarName:  "PULL_AUTHOR",
			EnvVarValue: validateEnvs.PullAuthor,
		},
		{
			StepName:    EnvStep,
			EnvVarName:  "HEAD_COMMIT",
			EnvVarValue: validateEnvs.HeadCommit,
		},
		{
			StepName:    EnvStep,
			EnvVarName:  "BASE_BRANCH_NAME",
			EnvVarValue: validateEnvs.BaseBranchName,
		},
		{
			StepName:    EnvStep,
			EnvVarName:  "ATLANTIS_TERRAFORM_VERSION",
			EnvVarValue: tfVersion,
		},
	}
	if t, ok := cfg.Tags[Manifest]; ok {
		//this is a Lyft specific env var
		steps = append(steps, workflows.PRStep{
			StepName:    EnvStep,
			EnvVarName:  "MANIFEST_FILEPATH",
			EnvVarValue: t,
		})
	}
	return steps
}

func (s *WorkflowSignaler) SignalWorkflow(ctx context.Context, workflowID string, runID string, signalName string, args interface{}) error {
	return s.TemporalClient.SignalWorkflow(ctx, workflowID, runID, signalName, args)
}
