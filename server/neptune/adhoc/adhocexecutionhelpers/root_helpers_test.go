package adhoc

import (
	"testing"

	v "github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/execute"
	"github.com/stretchr/testify/assert"
)

func TestPrependPlanEnvSteps(t *testing.T) {
	tests := []struct {
		cfg           *v.MergedProjectCfg
		expectedSteps []workflows.Step
	}{
		{
			cfg: &v.MergedProjectCfg{
				Tags: map[string]string{"manifest_path": "manifest_path"},
				DeploymentWorkflow: v.Workflow{
					Plan: v.Stage{
						Steps: []v.Step{
							{
								StepName:    "step1",
								ExtraArgs:   []string{"arg1", "arg2"},
								RunCommand:  "run",
								EnvVarName:  "env1",
								EnvVarValue: "value1",
							},
						},
					},
				},
			},
			expectedSteps: []workflows.Step{
				{
					StepName:    "env",
					EnvVarName:  "MANIFEST_FILEPATH",
					EnvVarValue: "manifest_path",
				},
				{
					StepName:    "step1",
					ExtraArgs:   []string{"arg1", "arg2"},
					RunCommand:  "run",
					EnvVarName:  "env1",
					EnvVarValue: "value1",
				},
			},
		},
		{
			cfg: &v.MergedProjectCfg{
				Tags: map[string]string{"foo": "foo"},
				DeploymentWorkflow: v.Workflow{
					Plan: v.Stage{
						Steps: []v.Step{
							{
								StepName:    "step1",
								ExtraArgs:   []string{"arg1", "arg2"},
								RunCommand:  "run",
								EnvVarName:  "env1",
								EnvVarValue: "value1",
							},
						},
					},
				},
			},
			expectedSteps: []workflows.Step{
				{
					StepName:    "step1",
					ExtraArgs:   []string{"arg1", "arg2"},
					RunCommand:  "run",
					EnvVarName:  "env1",
					EnvVarValue: "value1",
				},
			},
		},
	}

	for _, tt := range tests {
		res := prependPlanEnvSteps(tt.cfg)
		assert.True(t, compareSteps(res, tt.expectedSteps))
	}
}

func compareSteps(a []workflows.Step, b []workflows.Step) bool {
	if len(a) != len(b) {
		return false
	}

	for i, step := range a {
		if step.StepName != b[i].StepName {
			return false
		}
		if step.RunCommand != b[i].RunCommand {
			return false
		}
		if step.EnvVarName != b[i].EnvVarName {
			return false
		}
		if step.EnvVarValue != b[i].EnvVarValue {
			return false
		}
	}

	return true
}

func compareExecuteSteps(a []execute.Step, b []execute.Step) bool {
	if len(a) != len(b) {
		return false
	}

	for i, step := range a {
		if step.StepName != b[i].StepName {
			return false
		}
		if step.RunCommand != b[i].RunCommand {
			return false
		}
		if step.EnvVarName != b[i].EnvVarName {
			return false
		}
		if step.EnvVarValue != b[i].EnvVarValue {
			return false
		}
	}

	return true
}

func TestSteps(t *testing.T) {
	tests := []struct {
		requestSteps  []workflows.Step
		expectedSteps []execute.Step
	}{
		{
			requestSteps: []workflows.Step{
				{
					StepName:    "step1",
					ExtraArgs:   []string{"arg1", "arg2"},
					RunCommand:  "run",
					EnvVarName:  "env1",
					EnvVarValue: "value1",
				},
			},
			expectedSteps: []execute.Step{
				{
					StepName:    "step1",
					ExtraArgs:   []string{"arg1", "arg2"},
					RunCommand:  "run",
					EnvVarName:  "env1",
					EnvVarValue: "value1",
				},
			},
		},
	}

	for _, tt := range tests {
		res := steps(tt.requestSteps)
		assert.True(t, compareExecuteSteps(res, tt.expectedSteps))
	}
}
