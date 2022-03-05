package runtime

import (
	version "github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/core/runtime/policy"
	"github.com/runatlantis/atlantis/server/core/terraform"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/logging"
	lyftDecorators "github.com/runatlantis/atlantis/server/lyft/decorators"
)

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_steps_runner.go StepsRunner

// StepsRunner executes steps defined in project config
type StepsRunner interface {
	Run([]valid.Step, command.ProjectContext, string) ([]string, error)
}

func NewStepsRunner(
	terraformClient *terraform.DefaultClient,
	defaultTfVersion *version.Version,
	commitStatusUpdater StatusUpdater,
	binDir string,
	logger logging.SimpleLogging,
) (StepsRunner, error) {
	stepsRunner := &stepsRunner{}

	stepsRunner.initStepRunner = &InitStepRunner{
		TerraformExecutor: terraformClient,
		DefaultTFVersion:  defaultTfVersion,
	}

	planStepRunner := &PlanStepRunner{
		TerraformExecutor:   terraformClient,
		DefaultTFVersion:    defaultTfVersion,
		CommitStatusUpdater: commitStatusUpdater,
		AsyncTFExec:         terraformClient,
	}

	stepsRunner.planStepRunner = &lyftDecorators.DestroyPlanStepRunnerWrapper{
		StepRunner: planStepRunner,
	}

	showStepRunner, err := NewShowStepRunner(terraformClient, defaultTfVersion)
	if err != nil {
		return nil, errors.Wrap(err, "initializing show step runner")
	}
	stepsRunner.showStepRunner = showStepRunner

	policyCheckRunner, err := NewPolicyCheckStepRunner(
		defaultTfVersion,
		policy.NewConfTestExecutorWorkflow(logger, binDir, &terraform.DefaultDownloader{}),
	)
	if err != nil {
		return nil, errors.Wrap(err, "initializing policy check runner")
	}
	stepsRunner.policyCheckRunner = policyCheckRunner

	stepsRunner.applyStepRunner = &ApplyStepRunner{
		TerraformExecutor:   terraformClient,
		CommitStatusUpdater: commitStatusUpdater,
		AsyncTFExec:         terraformClient,
	}

	stepsRunner.versionStepRunner = &VersionStepRunner{
		TerraformExecutor: terraformClient,
		DefaultTFVersion:  defaultTfVersion,
	}

	runStepRunner := &RunStepRunner{
		TerraformExecutor: terraformClient,
		DefaultTFVersion:  defaultTfVersion,
		TerraformBinDir:   terraformClient.TerraformBinDir(),
	}

	stepsRunner.runStepRunner = runStepRunner

	stepsRunner.envStepRunner = &EnvStepRunner{
		RunStepRunner: runStepRunner,
	}

	return stepsRunner, nil
}

func (r *stepsRunner) Run(steps []valid.Step, ctx command.ProjectContext, absPath string) ([]string, error) {
	var outputs []string

	envs := make(map[string]string)
	for _, step := range steps {
		var out string
		var err error
		switch step.StepName {
		case "init":
			out, err = r.initStepRunner.Run(ctx, step.ExtraArgs, absPath, envs)
		case "plan":
			out, err = r.planStepRunner.Run(ctx, step.ExtraArgs, absPath, envs)
		case "show":
			_, err = r.showStepRunner.Run(ctx, step.ExtraArgs, absPath, envs)
		case "policy_check":
			out, err = r.policyCheckRunner.Run(ctx, step.ExtraArgs, absPath, envs)
		case "apply":
			out, err = r.applyStepRunner.Run(ctx, step.ExtraArgs, absPath, envs)
		case "version":
			out, err = r.versionStepRunner.Run(ctx, step.ExtraArgs, absPath, envs)
		case "run":
			out, err = r.runStepRunner.Run(ctx, step.RunCommand, absPath, envs)
		case "env":
			out, err = r.envStepRunner.Run(ctx, step.RunCommand, step.EnvVarValue, absPath, envs)
			envs[step.EnvVarName] = out
			// We reset out to the empty string because we don't want it to
			// be printed to the PR, it's solely to set the environment variable.
			out = ""
		}

		if out != "" {
			outputs = append(outputs, out)
		}
		if err != nil {
			return outputs, err
		}
	}
	return outputs, nil
}

type stepsRunner struct {
	initStepRunner    Runner
	planStepRunner    Runner
	showStepRunner    Runner
	policyCheckRunner Runner
	applyStepRunner   Runner
	versionStepRunner Runner
	envStepRunner     *EnvStepRunner
	runStepRunner     *RunStepRunner
}
