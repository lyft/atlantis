package runtime

import (
	"strings"

	version "github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/command"
	lyftRuntime "github.com/runatlantis/atlantis/server/lyft/core/runtime"
)

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_steps_runner.go StepsRunner

// StepsRunner executes steps defined in project config
type StepsRunner interface {
	Run(command.ProjectContext, string) (string, error)
}

func NewStepsRunner(
	terraformClient TerraformExec,
	terraformAsyncClient AsyncTFExec,
	defaultTfVersion *version.Version,
	commitStatusUpdater StatusUpdater,
	conftestExecutor VersionedExecutorWorkflow,
	binDir string,
) (*stepsRunner, error) {
	stepsRunner := &stepsRunner{}

	stepsRunner.InitStepRunner = &InitStepRunner{
		TerraformExecutor: terraformClient,
		DefaultTFVersion:  defaultTfVersion,
	}

	planStepRunner := &PlanStepRunner{
		TerraformExecutor:   terraformClient,
		DefaultTFVersion:    defaultTfVersion,
		CommitStatusUpdater: commitStatusUpdater,
		AsyncTFExec:         terraformAsyncClient,
	}

	stepsRunner.PlanStepRunner = &lyftRuntime.DestroyPlanStepRunner{
		StepRunner: planStepRunner,
	}

	showStepRunner, err := NewShowStepRunner(terraformClient, defaultTfVersion)
	if err != nil {
		return nil, errors.Wrap(err, "initializing show step runner")
	}
	stepsRunner.ShowStepRunner = showStepRunner

	policyCheckRunner, err := NewPolicyCheckStepRunner(
		defaultTfVersion,
		conftestExecutor,
	)
	if err != nil {
		return nil, errors.Wrap(err, "initializing policy check runner")
	}
	stepsRunner.PolicyCheckRunner = policyCheckRunner

	stepsRunner.ApplyStepRunner = &ApplyStepRunner{
		TerraformExecutor:   terraformClient,
		CommitStatusUpdater: commitStatusUpdater,
		AsyncTFExec:         terraformAsyncClient,
	}

	stepsRunner.VersionStepRunner = &VersionStepRunner{
		TerraformExecutor: terraformClient,
		DefaultTFVersion:  defaultTfVersion,
	}

	runStepRunner := &RunStepRunner{
		TerraformExecutor: terraformClient,
		DefaultTFVersion:  defaultTfVersion,
		TerraformBinDir:   binDir,
	}

	stepsRunner.RunStepRunner = runStepRunner

	stepsRunner.EnvStepRunner = &EnvStepRunner{
		RunStepRunner: runStepRunner,
	}

	return stepsRunner, nil
}

func (r *stepsRunner) Run(ctx command.ProjectContext, absPath string) (string, error) {
	var outputs []string

	envs := make(map[string]string)
	for _, step := range ctx.Steps {
		var out string
		var err error
		switch step.StepName {
		case "init":
			out, err = r.InitStepRunner.Run(ctx, step.ExtraArgs, absPath, envs)
		case "plan":
			out, err = r.PlanStepRunner.Run(ctx, step.ExtraArgs, absPath, envs)
		case "show":
			_, err = r.ShowStepRunner.Run(ctx, step.ExtraArgs, absPath, envs)
		case "policy_check":
			out, err = r.PolicyCheckRunner.Run(ctx, step.ExtraArgs, absPath, envs)
		case "apply":
			out, err = r.ApplyStepRunner.Run(ctx, step.ExtraArgs, absPath, envs)
		case "version":
			out, err = r.VersionStepRunner.Run(ctx, step.ExtraArgs, absPath, envs)
		case "run":
			out, err = r.RunStepRunner.Run(ctx, step.RunCommand, absPath, envs)
		case "env":
			out, err = r.EnvStepRunner.Run(ctx, step.RunCommand, step.EnvVarValue, absPath, envs)
			envs[step.EnvVarName] = out
			// We reset out to the empty string because we don't want it to
			// be printed to the PR, it's solely to set the environment variable.
			out = ""
		}

		if out != "" {
			outputs = append(outputs, out)
		}
		if err != nil {
			return strings.Join(outputs, "\n"), err
		}
	}
	return strings.Join(outputs, "\n"), nil
}

type stepsRunner struct {
	InitStepRunner    Runner
	PlanStepRunner    Runner
	ShowStepRunner    Runner
	PolicyCheckRunner Runner
	ApplyStepRunner   Runner
	VersionStepRunner Runner
	EnvStepRunner     EnvRunner
	RunStepRunner     CustomRunner
}
