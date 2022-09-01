package event_test

import (
	"fmt"
	"github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event"
	. "github.com/runatlantis/atlantis/testing"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

var globalConfig = valid.GlobalCfg{
	Repos: []valid.Repo{
		{
			IDRegex:              regexp.MustCompile(".*"),
			AllowCustomWorkflows: Bool(true),
			AllowedOverrides:     []string{"apply_requirements", "workflow"},
			CheckoutStrategy:     "branch",
		},
	},
}

func TestHasRepoCfg_DirDoesNotExist(t *testing.T) {
	r := event.ParserValidator{
		GlobalCfg: globalConfig,
	}
	_, err := r.ParseRepoCfg("/not/exist", "")
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestHasRepoCfg_FileDoesNotExist(t *testing.T) {
	tmpDir, cleanup := TempDir(t)
	defer cleanup()
	r := event.ParserValidator{
		GlobalCfg: globalConfig,
	}
	_, err := r.ParseRepoCfg(tmpDir, "")
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestHasRepoCfg_InvalidFileExtension(t *testing.T) {
	tmpDir, cleanup := TempDir(t)
	defer cleanup()
	_, err := os.Create(filepath.Join(tmpDir, "atlantis.yml"))
	assert.NoError(t, err)

	r := event.ParserValidator{
		GlobalCfg: globalConfig,
	}
	_, err = r.ParseRepoCfg(tmpDir, "")
	assert.ErrorContains(t, err, "found \"atlantis.yml\" as config file; rename using the .yaml extension - \"atlantis.yaml\"")
}

func TestParseRepoCfg_BadPermissions(t *testing.T) {
	tmpDir, cleanup := TempDir(t)
	defer cleanup()
	err := ioutil.WriteFile(filepath.Join(tmpDir, "atlantis.yaml"), nil, 0000)
	assert.NoError(t, err)

	r := event.ParserValidator{
		GlobalCfg: globalConfig,
	}
	_, err = r.ParseRepoCfg(tmpDir, "")
	assert.ErrorContains(t, err, "unable to read atlantis.yaml file: ")
}

// Test both ParseRepoCfg and ParseGlobalCfg when given in valid YAML.
// We only have a few cases here because we assume the YAML library to be
// well tested. See https://github.com/go-yaml/yaml/blob/v2/decode_test.go#L810.
func TestParseCfgs_InvalidYAML(t *testing.T) {
	cases := []struct {
		description string
		input       string
		expErr      string
	}{
		{
			"random characters",
			"slkjds",
			"yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `slkjds` into",
		},
		{
			"just a colon",
			":",
			"yaml: did not find expected key",
		},
	}

	tmpDir, cleanup := TempDir(t)
	defer cleanup()

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			confPath := filepath.Join(tmpDir, "atlantis.yaml")
			err := ioutil.WriteFile(confPath, []byte(c.input), 0600)
			assert.NoError(t, err)
			r := event.ParserValidator{
				GlobalCfg: globalConfig,
			}
			_, err = r.ParseRepoCfg(tmpDir, "")
			assert.ErrorContains(t, err, c.expErr)
		})
	}
}

func TestParseRepoCfg(t *testing.T) {
	tfVersion, _ := version.NewVersion("v0.11.0")
	cases := []struct {
		description string
		input       string
		expErr      string
		exp         valid.RepoCfg
	}{
		// Version key.
		{
			description: "no version",
			input: `
projects:
- dir: "."
`,
			expErr: "version: is required. If you've just upgraded Atlantis you need to rewrite your atlantis.yaml for version 3. See www.runatlantis.io/docs/upgrading-atlantis-yaml.html.",
		},
		{
			description: "unsupported version",
			input: `
version: 0
projects:
- dir: "."
`,
			expErr: "version: only versions 2 and 3 are supported.",
		},
		{
			description: "empty version",
			input: `
version:
projects:
- dir: "."
`,
			expErr: "version: is required. If you've just upgraded Atlantis you need to rewrite your atlantis.yaml for version 3. See www.runatlantis.io/docs/upgrading-atlantis-yaml.html.",
		},
		{
			description: "version 2",
			input: `
version: 2
workflows:
 custom:
   plan:
     steps:
     - run: old 'shell parsing'
`,
			exp: valid.RepoCfg{
				Version: 2,
				Workflows: map[string]valid.Workflow{
					"custom": {
						Name:        "custom",
						Apply:       valid.DefaultApplyStage,
						PolicyCheck: valid.DefaultPolicyCheckStage,
						Plan: valid.Stage{
							Steps: []valid.Step{
								{
									StepName:   "run",
									RunCommand: "old shell parsing",
								},
							},
						},
					},
				},
			},
		},

		// Projects key.
		{
			description: "empty projects list",
			input: `
version: 3
projects:`,
			exp: valid.RepoCfg{
				Version:   3,
				Projects:  nil,
				Workflows: map[string]valid.Workflow{},
			},
		},
		{
			description: "project dir not set",
			input: `
version: 3
projects:
- `,
			expErr: "projects: (0: (dir: cannot be blank.).).",
		},
		{
			description: "project dir set",
			input: `
version: 3
projects:
- dir: .`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:              ".",
						Workspace:        "default",
						WorkflowName:     nil,
						TerraformVersion: nil,
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      true,
						},
						ApplyRequirements: nil,
					},
				},
				Workflows: map[string]valid.Workflow{},
			},
		},
		{
			description: "autoplan should be enabled by default",
			input: `
version: 3
projects:
- dir: "."
`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:       ".",
						Workspace: "default",
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      true,
						},
					},
				},
				Workflows: make(map[string]valid.Workflow),
			},
		},
		{
			description: "autoplan should be enabled if only when_modified set",
			input: `
version: 3
projects:
- dir: "."
  autoplan:
    when_modified: ["**/*.tf*"]
`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:       ".",
						Workspace: "default",
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*"},
							Enabled:      true,
						},
					},
				},
				Workflows: make(map[string]valid.Workflow),
			},
		},
		{
			description: "if workflows not defined there are none",
			input: `
version: 3
projects:
- dir: "."
`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:       ".",
						Workspace: "default",
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      true,
						},
					},
				},
				Workflows: make(map[string]valid.Workflow),
			},
		},
		{
			description: "if workflows key set but with no workflows there are none",
			input: `
version: 3
projects:
- dir: "."
workflows: ~
`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:       ".",
						Workspace: "default",
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      true,
						},
					},
				},
				Workflows: make(map[string]valid.Workflow),
			},
		},
		{
			description: "if a plan or apply explicitly defines an empty steps key then it gets the defaults",
			input: `
version: 3
projects:
- dir: "."
workflows:
 default:
   plan:
     steps:
   apply:
     steps:
`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:       ".",
						Workspace: "default",
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      true,
						},
					},
				},
				Workflows: map[string]valid.Workflow{
					"default": {
						Name:        "default",
						Plan:        valid.DefaultPlanStage,
						Apply:       valid.DefaultApplyStage,
						PolicyCheck: valid.DefaultPolicyCheckStage,
					},
				},
			},
		},
		{
			description: "project fields set except autoplan",
			input: `
version: 3
projects:
- dir: .
  workspace: myworkspace
  terraform_version: v0.11.0
  apply_requirements: [approved]
  workflow: myworkflow
workflows:
  myworkflow: ~`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:              ".",
						Workspace:        "myworkspace",
						WorkflowName:     String("myworkflow"),
						TerraformVersion: tfVersion,
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      true,
						},
						ApplyRequirements: []string{"approved"},
					},
				},
				Workflows: map[string]valid.Workflow{
					"myworkflow": {
						Name:        "myworkflow",
						Apply:       valid.DefaultApplyStage,
						Plan:        valid.DefaultPlanStage,
						PolicyCheck: valid.DefaultPolicyCheckStage,
					},
				},
			},
		},
		{
			description: "project field with autoplan",
			input: `
version: 3
projects:
- dir: .
  workspace: myworkspace
  terraform_version: v0.11.0
  apply_requirements: [approved]
  workflow: myworkflow
  autoplan:
    enabled: false
workflows:
  myworkflow: ~`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:              ".",
						Workspace:        "myworkspace",
						WorkflowName:     String("myworkflow"),
						TerraformVersion: tfVersion,
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      false,
						},
						ApplyRequirements: []string{"approved"},
					},
				},
				Workflows: map[string]valid.Workflow{
					"myworkflow": {
						Name:        "myworkflow",
						Apply:       valid.DefaultApplyStage,
						Plan:        valid.DefaultPlanStage,
						PolicyCheck: valid.DefaultPolicyCheckStage,
					},
				},
			},
		},
		{
			description: "project field with mergeable apply requirement",
			input: `
version: 3
projects:
- dir: .
  workspace: myworkspace
  terraform_version: v0.11.0
  apply_requirements: [mergeable]
  workflow: myworkflow
  autoplan:
    enabled: false
workflows:
  myworkflow: ~`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:              ".",
						Workspace:        "myworkspace",
						WorkflowName:     String("myworkflow"),
						TerraformVersion: tfVersion,
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      false,
						},
						ApplyRequirements: []string{"mergeable"},
					},
				},
				Workflows: map[string]valid.Workflow{
					"myworkflow": {
						Name:        "myworkflow",
						Apply:       valid.DefaultApplyStage,
						Plan:        valid.DefaultPlanStage,
						PolicyCheck: valid.DefaultPolicyCheckStage,
					},
				},
			},
		},
		{
			description: "project field with undiverged apply requirement",
			input: `
version: 3
projects:
- dir: .
  workspace: myworkspace
  terraform_version: v0.11.0
  apply_requirements: [undiverged]
  workflow: myworkflow
  autoplan:
    enabled: false
workflows:
  myworkflow: ~`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:              ".",
						Workspace:        "myworkspace",
						WorkflowName:     String("myworkflow"),
						TerraformVersion: tfVersion,
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      false,
						},
						ApplyRequirements: []string{"undiverged"},
					},
				},
				Workflows: map[string]valid.Workflow{
					"myworkflow": {
						Name:        "myworkflow",
						Apply:       valid.DefaultApplyStage,
						Plan:        valid.DefaultPlanStage,
						PolicyCheck: valid.DefaultPolicyCheckStage,
					},
				},
			},
		},
		{
			description: "project field with mergeable and approved apply requirements",
			input: `
version: 3
projects:
- dir: .
  workspace: myworkspace
  terraform_version: v0.11.0
  apply_requirements: [mergeable, approved]
  workflow: myworkflow
  autoplan:
    enabled: false
workflows:
  myworkflow: ~`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:              ".",
						Workspace:        "myworkspace",
						WorkflowName:     String("myworkflow"),
						TerraformVersion: tfVersion,
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      false,
						},
						ApplyRequirements: []string{"mergeable", "approved"},
					},
				},
				Workflows: map[string]valid.Workflow{
					"myworkflow": {
						Name:        "myworkflow",
						Apply:       valid.DefaultApplyStage,
						Plan:        valid.DefaultPlanStage,
						PolicyCheck: valid.DefaultPolicyCheckStage,
					},
				},
			},
		},
		{
			description: "project field with undiverged and approved apply requirements",
			input: `
version: 3
projects:
- dir: .
  workspace: myworkspace
  terraform_version: v0.11.0
  apply_requirements: [undiverged, approved]
  workflow: myworkflow
  autoplan:
    enabled: false
workflows:
  myworkflow: ~`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:              ".",
						Workspace:        "myworkspace",
						WorkflowName:     String("myworkflow"),
						TerraformVersion: tfVersion,
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      false,
						},
						ApplyRequirements: []string{"undiverged", "approved"},
					},
				},
				Workflows: map[string]valid.Workflow{
					"myworkflow": {
						Name:        "myworkflow",
						Apply:       valid.DefaultApplyStage,
						Plan:        valid.DefaultPlanStage,
						PolicyCheck: valid.DefaultPolicyCheckStage,
					},
				},
			},
		},
		{
			description: "project field with undiverged and mergeable apply requirements",
			input: `
version: 3
projects:
- dir: .
  workspace: myworkspace
  terraform_version: v0.11.0
  apply_requirements: [undiverged, mergeable]
  workflow: myworkflow
  autoplan:
    enabled: false
workflows:
  myworkflow: ~`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:              ".",
						Workspace:        "myworkspace",
						WorkflowName:     String("myworkflow"),
						TerraformVersion: tfVersion,
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      false,
						},
						ApplyRequirements: []string{"undiverged", "mergeable"},
					},
				},
				Workflows: map[string]valid.Workflow{
					"myworkflow": {
						Name:        "myworkflow",
						Apply:       valid.DefaultApplyStage,
						Plan:        valid.DefaultPlanStage,
						PolicyCheck: valid.DefaultPolicyCheckStage,
					},
				},
			},
		},
		{
			description: "project field with undiverged, mergeable and approved apply requirements",
			input: `
version: 3
projects:
- dir: .
  workspace: myworkspace
  terraform_version: v0.11.0
  apply_requirements: [undiverged, mergeable, approved]
  workflow: myworkflow
  autoplan:
    enabled: false
workflows:
  myworkflow: ~`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:              ".",
						Workspace:        "myworkspace",
						WorkflowName:     String("myworkflow"),
						TerraformVersion: tfVersion,
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      false,
						},
						ApplyRequirements: []string{"undiverged", "mergeable", "approved"},
					},
				},
				Workflows: map[string]valid.Workflow{
					"myworkflow": {
						Name:        "myworkflow",
						Apply:       valid.DefaultApplyStage,
						Plan:        valid.DefaultPlanStage,
						PolicyCheck: valid.DefaultPolicyCheckStage,
					},
				},
			},
		},
		{
			description: "project dir with ..",
			input: `
version: 3
projects:
- dir: ..`,
			expErr: "projects: (0: (dir: cannot contain '..'.).).",
		},

		// Project must have dir set.
		{
			description: "project with no config",
			input: `
version: 3
projects:
-`,
			expErr: "projects: (0: (dir: cannot be blank.).).",
		},
		{
			description: "project with no config at index 1",
			input: `
version: 3
projects:
- dir: "."
-`,
			expErr: "projects: (1: (dir: cannot be blank.).).",
		},
		{
			description: "project with unknown key",
			input: `
version: 3
projects:
- unknown: value`,
			expErr: "yaml: unmarshal errors:\n  line 4: field unknown not found in type raw.Project",
		},
		{
			description: "referencing workflow that doesn't exist",
			input: `
version: 3
projects:
- dir: .
  workflow: undefined`,
			expErr: "workflow \"undefined\" is not defined anywhere",
		},
		{
			description: "two projects with same dir/workspace without names",
			input: `
version: 3
projects:
- dir: .
  workspace: workspace
- dir: .
  workspace: workspace`,
			expErr: "there are two or more projects with dir: \".\" workspace: \"workspace\" that are not all named; they must have a 'name' key so they can be targeted for apply's separately",
		},
		{
			description: "two projects with same dir/workspace only one with name",
			input: `
version: 3
projects:
- name: myname
  dir: .
  workspace: workspace
- dir: .
  workspace: workspace`,
			expErr: "there are two or more projects with dir: \".\" workspace: \"workspace\" that are not all named; they must have a 'name' key so they can be targeted for apply's separately",
		},
		{
			description: "two projects with same dir/workspace both with same name",
			input: `
version: 3
projects:
- name: myname
  dir: .
  workspace: workspace
- name: myname
  dir: .
  workspace: workspace`,
			expErr: "found two or more projects with name \"myname\"; project names must be unique",
		},
		{
			description: "two projects with same dir/workspace with different names",
			input: `
version: 3
projects:
- name: myname
  dir: .
  workspace: workspace
- name: myname2
  dir: .
  workspace: workspace`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Name:      String("myname"),
						Dir:       ".",
						Workspace: "workspace",
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      true,
						},
					},
					{
						Name:      String("myname2"),
						Dir:       ".",
						Workspace: "workspace",
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      true,
						},
					},
				},
				Workflows: map[string]valid.Workflow{},
			},
		},
		{
			description: "if steps are set then we parse them properly",
			input: `
version: 3
projects:
- dir: "."
workflows:
 default:
   plan:
     steps:
     - init
     - plan
   policy_check:
     steps:
     - init
     - policy_check
   apply:
     steps:
     - plan # NOTE: we don't validate if they make sense
     - apply
`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:       ".",
						Workspace: "default",
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      true,
						},
					},
				},
				Workflows: map[string]valid.Workflow{
					"default": {
						Name: "default",
						Plan: valid.Stage{
							Steps: []valid.Step{
								{
									StepName: "init",
								},
								{
									StepName: "plan",
								},
							},
						},
						PolicyCheck: valid.Stage{
							Steps: []valid.Step{
								{
									StepName: "init",
								},
								{
									StepName: "policy_check",
								},
							},
						},
						Apply: valid.Stage{
							Steps: []valid.Step{
								{
									StepName: "plan",
								},
								{
									StepName: "apply",
								},
							},
						},
					},
				},
			},
		},
		{
			description: "we parse extra_args for the steps",
			input: `
version: 3
projects:
- dir: "."
workflows:
 default:
   plan:
     steps:
     - init:
         extra_args: []
     - plan:
         extra_args:
         - arg1
         - arg2
   policy_check:
     steps:
     - policy_check:
         extra_args:
         - arg1
   apply:
     steps:
     - plan:
         extra_args: [a, b]
     - apply:
         extra_args: ["a", "b"]
`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:       ".",
						Workspace: "default",
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      true,
						},
					},
				},
				Workflows: map[string]valid.Workflow{
					"default": {
						Name: "default",
						Plan: valid.Stage{
							Steps: []valid.Step{
								{
									StepName:  "init",
									ExtraArgs: []string{},
								},
								{
									StepName:  "plan",
									ExtraArgs: []string{"arg1", "arg2"},
								},
							},
						},
						PolicyCheck: valid.Stage{
							Steps: []valid.Step{
								{
									StepName:  "policy_check",
									ExtraArgs: []string{"arg1"},
								},
							},
						},
						Apply: valid.Stage{
							Steps: []valid.Step{
								{
									StepName:  "plan",
									ExtraArgs: []string{"a", "b"},
								},
								{
									StepName:  "apply",
									ExtraArgs: []string{"a", "b"},
								},
							},
						},
					},
				},
			},
		},
		{
			description: "custom steps are parsed",
			input: `
version: 3
projects:
- dir: "."
workflows:
 default:
   plan:
     steps:
     - run: "echo \"plan hi\""
   policy_check:
     steps:
     - run: "echo \"opa hi\""
   apply:
     steps:
     - run: echo apply "arg 2"
`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:       ".",
						Workspace: "default",
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      true,
						},
					},
				},
				Workflows: map[string]valid.Workflow{
					"default": {
						Name: "default",
						Plan: valid.Stage{
							Steps: []valid.Step{
								{
									StepName:   "run",
									RunCommand: "echo \"plan hi\"",
								},
							},
						},
						PolicyCheck: valid.Stage{
							Steps: []valid.Step{
								{
									StepName:   "run",
									RunCommand: "echo \"opa hi\"",
								},
							},
						},
						Apply: valid.Stage{
							Steps: []valid.Step{
								{
									StepName:   "run",
									RunCommand: "echo apply \"arg 2\"",
								},
							},
						},
					},
				},
			},
		},
		{
			description: "env steps",
			input: `
version: 3
projects:
- dir: "."
workflows:
 default:
   plan:
     steps:
     - env:
         name: env_name
         value: env_value
   policy_check:
     steps:
     - env:
         name: env_name
         value: env_value
   apply:
     steps:
     - env:
         name: env_name
         command: command and args
`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:       ".",
						Workspace: "default",
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      true,
						},
					},
				},
				Workflows: map[string]valid.Workflow{
					"default": {
						Name: "default",
						Plan: valid.Stage{
							Steps: []valid.Step{
								{
									StepName:    "env",
									EnvVarName:  "env_name",
									EnvVarValue: "env_value",
								},
							},
						},
						PolicyCheck: valid.Stage{
							Steps: []valid.Step{
								{
									StepName:    "env",
									EnvVarName:  "env_name",
									EnvVarValue: "env_value",
								},
							},
						},
						Apply: valid.Stage{
							Steps: []valid.Step{
								{
									StepName:   "env",
									EnvVarName: "env_name",
									RunCommand: "command and args",
								},
							},
						},
					},
				},
			},
		},
	}

	tmpDir, cleanup := TempDir(t)
	defer cleanup()

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			err := ioutil.WriteFile(filepath.Join(tmpDir, "atlantis.yaml"), []byte(c.input), 0600)
			assert.NoError(t, err)

			r := event.ParserValidator{
				GlobalCfg: globalConfig,
			}
			act, err := r.ParseRepoCfg(tmpDir, "")
			if c.expErr != "" {
				assert.ErrorContains(t, err, c.expErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, c.exp, act)
		})
	}
}

// Test that we fail if the global validation fails. We test global validation
// more completely in GlobalCfg.ValidateRepoCfg().
func TestParseRepoCfg_GlobalValidation(t *testing.T) {
	tmpDir, cleanup := TempDir(t)
	defer cleanup()

	repoCfg := `
version: 3
projects:
- dir: .
  workflow: custom
workflows:
  custom: ~`
	err := ioutil.WriteFile(filepath.Join(tmpDir, "atlantis.yaml"), []byte(repoCfg), 0600)
	assert.NoError(t, err)

	r := event.ParserValidator{
		GlobalCfg: valid.NewGlobalCfg(),
	}

	_, err = r.ParseRepoCfg(tmpDir, "repo_id")
	assert.ErrorContains(t, err, "repo config not allowed to set 'workflow' key: server-side config needs 'allowed_overrides: [workflow]'")
}

// Test legacy shell parsing vs v3 parsing.
func TestParseRepoCfg_V2ShellParsing(t *testing.T) {
	cases := []struct {
		in       string
		expV2    string
		expV2Err string
	}{
		{
			in:    "echo a b",
			expV2: "echo a b",
		},
		{
			in:    "echo 'a b'",
			expV2: "echo a b",
		},
		{
			in:       "echo 'a b",
			expV2Err: "unable to parse \"echo 'a b\": EOF found when expecting closing quote.",
		},
		{
			in:    `mkdir a/b/c || printf \'your main.tf file does not provide default region.\\ncheck\'`,
			expV2: `mkdir a/b/c || printf 'your main.tf file does not provide default region.\ncheck'`,
		},
	}

	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			v2Dir, cleanup2 := TempDir(t)
			defer cleanup2()
			v3Dir, cleanup3 := TempDir(t)
			defer cleanup3()
			v2Path := filepath.Join(v2Dir, "atlantis.yaml")
			v3Path := filepath.Join(v3Dir, "atlantis.yaml")
			cfg := fmt.Sprintf(`workflows:
 custom:
   plan:
     steps:
     - run: %s
   apply:
     steps:
     - run: %s`, c.in, c.in)
			assert.NoError(t, ioutil.WriteFile(v2Path, []byte("version: 2\n"+cfg), 0600))
			assert.NoError(t, ioutil.WriteFile(v3Path, []byte("version: 3\n"+cfg), 0600))

			p := &event.ParserValidator{
				GlobalCfg: globalConfig,
			}

			v2Cfg, err := p.ParseRepoCfg(v2Dir, "")
			if c.expV2Err != "" {
				assert.ErrorContains(t, err, c.expV2Err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, c.expV2, v2Cfg.Workflows["custom"].Plan.Steps[0].RunCommand)
				assert.Equal(t, c.expV2, v2Cfg.Workflows["custom"].Apply.Steps[0].RunCommand)
			}
			v3Cfg, err := p.ParseRepoCfg(v3Dir, "")
			assert.NoError(t, err)
			assert.Equal(t, c.in, v3Cfg.Workflows["custom"].Plan.Steps[0].RunCommand)
			assert.Equal(t, c.in, v3Cfg.Workflows["custom"].Apply.Steps[0].RunCommand)
		})
	}
}
