package config_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/runatlantis/atlantis/server/config/raw"

	"github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/config"
	"github.com/runatlantis/atlantis/server/config/valid"
	. "github.com/runatlantis/atlantis/testing"
)

var globalCfg = valid.GlobalCfg{
	Repos: []valid.Repo{
		{
			IDRegex:          regexp.MustCompile(".*"),
			AllowedOverrides: []string{"apply_requirements", "pull_request_workflow"},
			CheckoutStrategy: "branch",
			ApplySettings: valid.ApplySettings{
				BranchRestriction: valid.DefaultBranchRestriction,
			},
		},
	},
	PullRequestWorkflows: map[string]valid.Workflow{
		"myworkflow": {},
	},
}

func TestHasRepoCfg_DirDoesNotExist(t *testing.T) {
	r := config.ParserValidator{}
	exists, err := r.HasRepoCfg("/not/exist")
	Ok(t, err)
	Equals(t, false, exists)
}

func TestHasRepoCfg_FileDoesNotExist(t *testing.T) {
	tmpDir, cleanup := TempDir(t)
	defer cleanup()
	r := config.ParserValidator{}
	exists, err := r.HasRepoCfg(tmpDir)
	Ok(t, err)
	Equals(t, false, exists)
}

func TestHasRepoCfg_InvalidFileExtension(t *testing.T) {
	tmpDir, cleanup := TempDir(t)
	defer cleanup()
	_, err := os.Create(filepath.Join(tmpDir, "atlantis.yml"))
	Ok(t, err)

	r := config.ParserValidator{}
	_, err = r.HasRepoCfg(tmpDir)
	ErrContains(t, "found \"atlantis.yml\" as config file; rename using the .yaml extension - \"atlantis.yaml\"", err)
}

func TestParseRepoCfg_DirDoesNotExist(t *testing.T) {
	r := config.ParserValidator{}
	_, err := r.ParseRepoCfg("/not/exist", globalCfg, "")
	Assert(t, os.IsNotExist(err), "exp not exist err")
}

func TestParseRepoCfg_FileDoesNotExist(t *testing.T) {
	tmpDir, cleanup := TempDir(t)
	defer cleanup()
	r := config.ParserValidator{}
	_, err := r.ParseRepoCfg(tmpDir, globalCfg, "")
	Assert(t, os.IsNotExist(err), "exp not exist err")
}

func TestParseRepoCfg_BadPermissions(t *testing.T) {
	tmpDir, cleanup := TempDir(t)
	defer cleanup()
	err := os.WriteFile(filepath.Join(tmpDir, "atlantis.yaml"), nil, 0000)
	Ok(t, err)

	r := config.ParserValidator{}
	_, err = r.ParseRepoCfg(tmpDir, globalCfg, "")
	ErrContains(t, "unable to read atlantis.yaml file: ", err)
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
			err := os.WriteFile(confPath, []byte(c.input), 0600)
			Ok(t, err)
			r := config.ParserValidator{}
			_, err = r.ParseRepoCfg(tmpDir, globalCfg, "")
			ErrContains(t, c.expErr, err)
			_, err = r.ParseGlobalCfg(confPath, valid.NewGlobalCfg("somedir"))
			ErrContains(t, c.expErr, err)
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

		// Projects key.
		{
			description: "empty projects list",
			input: `
version: 3
projects:`,
			exp: valid.RepoCfg{
				Version:  3,
				Projects: nil,
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
						Dir:                     ".",
						Workspace:               "default",
						PullRequestWorkflowName: nil,
						TerraformVersion:        nil,
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      true,
						},
						ApplyRequirements: nil,
					},
				},
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
			},
		},
		{
			description: "if workflows key set but with no workflows there are none",
			input: `
version: 3
projects:
- dir: "."`,
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
			},
		},
		{
			description: "if a plan or apply explicitly defines an empty steps key then it gets the defaults",
			input: `
version: 3
projects:
- dir: "."`,
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
  pull_request_workflow: myworkflow`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:                     ".",
						Workspace:               "myworkspace",
						PullRequestWorkflowName: String("myworkflow"),
						TerraformVersion:        tfVersion,
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      true,
						},
						ApplyRequirements: []string{"approved"},
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
  pull_request_workflow: myworkflow
  autoplan:
    enabled: false`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:                     ".",
						Workspace:               "myworkspace",
						PullRequestWorkflowName: String("myworkflow"),
						TerraformVersion:        tfVersion,
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      false,
						},
						ApplyRequirements: []string{"approved"},
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
  pull_request_workflow: myworkflow
  autoplan:
    enabled: false`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:                     ".",
						Workspace:               "myworkspace",
						PullRequestWorkflowName: String("myworkflow"),
						TerraformVersion:        tfVersion,
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      false,
						},
						ApplyRequirements: []string{"mergeable"},
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
  pull_request_workflow: myworkflow
  autoplan:
    enabled: false`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:                     ".",
						Workspace:               "myworkspace",
						PullRequestWorkflowName: String("myworkflow"),
						TerraformVersion:        tfVersion,
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      false,
						},
						ApplyRequirements: []string{"undiverged"},
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
  pull_request_workflow: myworkflow
  autoplan:
    enabled: false`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:                     ".",
						Workspace:               "myworkspace",
						PullRequestWorkflowName: String("myworkflow"),
						TerraformVersion:        tfVersion,
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      false,
						},
						ApplyRequirements: []string{"mergeable", "approved"},
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
  pull_request_workflow: myworkflow
  autoplan:
    enabled: false`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:                     ".",
						Workspace:               "myworkspace",
						PullRequestWorkflowName: String("myworkflow"),
						TerraformVersion:        tfVersion,
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      false,
						},
						ApplyRequirements: []string{"undiverged", "approved"},
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
  pull_request_workflow: myworkflow
  autoplan:
    enabled: false`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:                     ".",
						Workspace:               "myworkspace",
						PullRequestWorkflowName: String("myworkflow"),
						TerraformVersion:        tfVersion,
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      false,
						},
						ApplyRequirements: []string{"undiverged", "mergeable"},
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
  pull_request_workflow: myworkflow
  autoplan:
    enabled: false`,
			exp: valid.RepoCfg{
				Version: 3,
				Projects: []valid.Project{
					{
						Dir:                     ".",
						Workspace:               "myworkspace",
						PullRequestWorkflowName: String("myworkflow"),
						TerraformVersion:        tfVersion,
						Autoplan: valid.Autoplan{
							WhenModified: []string{"**/*.tf*", "**/terragrunt.hcl"},
							Enabled:      false,
						},
						ApplyRequirements: []string{"undiverged", "mergeable", "approved"},
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
  pull_request_workflow: undefined`,
			expErr: "pull_request_workflow \"undefined\" is not defined anywhere",
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
			},
		},
		{
			description: "if steps are set then we parse them properly",
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
			},
		},
		{
			description: "we parse extra_args for the steps",
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
			},
		},
		{
			description: "custom steps are parsed",
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
			},
		},
		{
			description: "env steps",
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
			},
		},
	}

	tmpDir, cleanup := TempDir(t)
	defer cleanup()

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			err := os.WriteFile(filepath.Join(tmpDir, "atlantis.yaml"), []byte(c.input), 0600)
			Ok(t, err)

			r := config.ParserValidator{}
			act, err := r.ParseRepoCfg(tmpDir, globalCfg, "")
			if c.expErr != "" {
				ErrEquals(t, c.expErr, err)
				return
			}
			Ok(t, err)
			Equals(t, c.exp, act)
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
  pull_request_workflow: custom`
	err := os.WriteFile(filepath.Join(tmpDir, "atlantis.yaml"), []byte(repoCfg), 0600)
	Ok(t, err)

	r := config.ParserValidator{}

	_, err = r.ParseRepoCfg(tmpDir, valid.NewGlobalCfg("somedir"), "repo_id")
	ErrEquals(t, "repo config not allowed to set 'pull_request_workflow' key: server-side config needs 'allowed_overrides: [pull_request_workflow]'", err)
}

func TestParseGlobalCfg_NotExist(t *testing.T) {
	r := config.ParserValidator{}
	_, err := r.ParseGlobalCfg("/not/exist", valid.NewGlobalCfg("somedir"))
	ErrEquals(t, "unable to read /not/exist file: open /not/exist: no such file or directory", err)
}

func TestParseGlobalCfg(t *testing.T) {
	defaultCfg := valid.NewGlobalCfg("somedir")
	preWorkflowHook := &valid.PreWorkflowHook{
		StepName:   "run",
		RunCommand: "custom workflow command",
	}
	preWorkflowHooks := []*valid.PreWorkflowHook{preWorkflowHook}
	defaultCfg.Temporal.TerraformTaskQueue = raw.DefaultTaskqueue

	customWorkflow1 := valid.Workflow{
		Name: "custom1",
		Plan: valid.Stage{
			Steps: []valid.Step{
				{
					StepName:   "run",
					RunCommand: "custom command",
				},
				{
					StepName:  "init",
					ExtraArgs: []string{"extra", "args"},
				},
				{
					StepName: "plan",
				},
			},
		},
		PolicyCheck: valid.Stage{
			Steps: []valid.Step{
				{
					StepName:   "run",
					RunCommand: "custom command",
				},
				{
					StepName:  "plan",
					ExtraArgs: []string{"extra", "args"},
				},
				{
					StepName: "policy_check",
				},
			},
		},
	}

	conftestVersion, _ := version.NewVersion("v1.0.0")

	cases := map[string]struct {
		input  string
		expErr string
		exp    valid.GlobalCfg
	}{
		"empty file": {
			input:  "",
			expErr: "file <tmp> was empty",
		},
		"invalid fields": {
			input:  "invalid: key",
			expErr: "yaml: unmarshal errors:\n  line 1: field invalid not found in type raw.GlobalCfg",
		},
		"no id specified": {
			input: `repos:
- apply_requirements: []`,
			expErr: "repos: (0: (id: cannot be blank.).).",
		},
		"invalid id regex": {
			input: `repos:
- id: /?/`,
			expErr: "repos: (0: (id: parsing: /?/: error parsing regexp: missing argument to repetition operator: `?`.).).",
		},
		"invalid branch regex": {
			input: `repos:
- id: /.*/
  branch: /?/`,
			expErr: "repos: (0: (branch: parsing: /?/: error parsing regexp: missing argument to repetition operator: `?`.).).",
		},
		"workflow doesn't exist": {
			input: `repos:
- id: /.*/
  pull_request_workflow: notdefined`,
			expErr: "workflow \"notdefined\" is not defined",
		},
		"invalid allowed_override": {
			input: `repos:
- id: /.*/
  allowed_overrides: [invalid]`,
			expErr: "repos: (0: (allowed_overrides: \"invalid\" is not a valid override, only \"apply_requirements\" and \"workflow\" are supported.).).",
		},
		"invalid apply_requirement": {
			input: `repos:
- id: /.*/
  apply_requirements: [invalid]`,
			expErr: "repos: (0: (apply_requirements: \"invalid\" is not a valid apply_requirement, supported apply requirements are: \"approved\", \"mergeable\", \"undiverged\", \"unlocked\".).).",
		},
		"no workflows key": {
			input: `repos: []`,
			exp:   defaultCfg,
		},
		"workflows empty": {
			input: `pull_request_workflows:`,
			exp:   defaultCfg,
		},
		"apply settings": {
			input: `repos:
- id: github.com/owner/repo
  apply_settings:
    pr_requirements: [approved]
    branch_restriction: none
    team: some_team`,
			exp: valid.GlobalCfg{
				Repos: []valid.Repo{
					defaultCfg.Repos[0],
					{
						ID:               "github.com/owner/repo",
						CheckoutStrategy: "branch",
						ApplySettings: valid.ApplySettings{
							PRRequirements:    []string{"approved"},
							BranchRestriction: valid.NoBranchRestriction,
							Team:              "some_team",
						},
					},
				},
				DeploymentWorkflows:  defaultCfg.DeploymentWorkflows,
				PullRequestWorkflows: defaultCfg.PullRequestWorkflows,
				Temporal:             valid.Temporal{TerraformTaskQueue: raw.DefaultTaskqueue},
				PersistenceConfig:    defaultCfg.PersistenceConfig,
			},
		},
		"workflow name but the rest is empty": {
			input: `
pull_request_workflows:
  name:`,
			exp: valid.GlobalCfg{
				Repos: defaultCfg.Repos,
				PullRequestWorkflows: map[string]valid.Workflow{
					"default": defaultCfg.PullRequestWorkflows["default"],
					"name": {
						Name:        "name",
						Plan:        valid.DefaultPlanStage,
						PolicyCheck: valid.DefaultPolicyCheckStage,
					},
				},
				DeploymentWorkflows: map[string]valid.Workflow{
					"default": defaultCfg.DeploymentWorkflows["default"],
				},
				Temporal:          valid.Temporal{TerraformTaskQueue: raw.DefaultTaskqueue},
				PersistenceConfig: defaultCfg.PersistenceConfig,
			},
		},
		"workflow stages empty": {
			input: `
pull_request_workflows:
  name:
    plan:
`,
			exp: valid.GlobalCfg{
				Repos: defaultCfg.Repos,
				PullRequestWorkflows: map[string]valid.Workflow{
					"default": defaultCfg.PullRequestWorkflows["default"],
					"name": {
						Name:        "name",
						Plan:        valid.DefaultPlanStage,
						PolicyCheck: valid.DefaultPolicyCheckStage,
					},
				},
				DeploymentWorkflows: map[string]valid.Workflow{
					"default": defaultCfg.DeploymentWorkflows["default"],
				},
				Temporal:          valid.Temporal{TerraformTaskQueue: raw.DefaultTaskqueue},
				PersistenceConfig: defaultCfg.PersistenceConfig,
			},
		},
		"workflow steps empty": {
			input: `
pull_request_workflows:
  name:
    plan:
      steps:`,
			exp: valid.GlobalCfg{
				Repos: defaultCfg.Repos,
				PullRequestWorkflows: map[string]valid.Workflow{
					"default": defaultCfg.PullRequestWorkflows["default"],
					"name": {
						Name:        "name",
						Plan:        valid.DefaultPlanStage,
						PolicyCheck: valid.DefaultPolicyCheckStage,
					},
				},
				DeploymentWorkflows: map[string]valid.Workflow{
					"default": defaultCfg.DeploymentWorkflows["default"],
				},
				Temporal:          valid.Temporal{TerraformTaskQueue: raw.DefaultTaskqueue},
				PersistenceConfig: defaultCfg.PersistenceConfig,
			},
		},
		"all keys specified": {
			input: `
repos:
- id: github.com/owner/repo
  apply_requirements: [approved, mergeable]
  pre_workflow_hooks:
    - run: custom workflow command
  allowed_overrides: [apply_requirements, workflow]
  checkout_strategy: merge
- id: /.*/
  branch: /(master|main)/
  pre_workflow_hooks:
    - run: custom workflow command
pull_request_workflows:
  custom1:
    plan:
      steps:
      - run: custom command
      - init:
          extra_args: [extra, args]
      - plan
    policy_check:
      steps:
      - run: custom command
      - plan:
          extra_args: [extra, args]
      - policy_check
policies:
  conftest_version: v1.0.0
  policy_sets:
    - name: good-policy
      paths: [rel/path/to/policy]
`,
			exp: valid.GlobalCfg{
				Repos: []valid.Repo{
					defaultCfg.Repos[0],
					{
						ID:                "github.com/owner/repo",
						ApplyRequirements: []string{"approved", "mergeable", "policies_passed"},
						PreWorkflowHooks:  preWorkflowHooks,
						AllowedOverrides:  []string{"apply_requirements", "workflow"},
						CheckoutStrategy:  "merge",
						ApplySettings: valid.ApplySettings{
							BranchRestriction: valid.DefaultBranchRestriction,
						},
					},
					{
						IDRegex:           regexp.MustCompile(".*"),
						BranchRegex:       regexp.MustCompile("(master|main)"),
						ApplyRequirements: []string{"policies_passed"},
						PreWorkflowHooks:  preWorkflowHooks,
						CheckoutStrategy:  "branch",
						ApplySettings: valid.ApplySettings{
							BranchRestriction: valid.DefaultBranchRestriction,
						},
					},
				},
				PullRequestWorkflows: map[string]valid.Workflow{
					"default": defaultCfg.PullRequestWorkflows["default"],
					"custom1": customWorkflow1,
				},
				PolicySets: valid.PolicySets{
					Version: conftestVersion,
					PolicySets: []valid.PolicySet{
						{
							Name:  "good-policy",
							Paths: []string{"rel/path/to/policy"},
						},
					},
				},
				DeploymentWorkflows: map[string]valid.Workflow{
					"default": defaultCfg.DeploymentWorkflows["default"],
				},
				Temporal:          valid.Temporal{TerraformTaskQueue: raw.DefaultTaskqueue},
				PersistenceConfig: defaultCfg.PersistenceConfig,
			},
		},
		"id regex with trailing slash": {
			input: `
repos:
- id: /github.com//
`,
			exp: valid.GlobalCfg{
				Repos: []valid.Repo{
					defaultCfg.Repos[0],
					{
						IDRegex:          regexp.MustCompile("github.com/"),
						CheckoutStrategy: "branch",
						ApplySettings: valid.ApplySettings{
							BranchRestriction: valid.DefaultBranchRestriction,
						},
					},
				},
				PullRequestWorkflows: map[string]valid.Workflow{
					"default": defaultCfg.PullRequestWorkflows["default"],
				},
				DeploymentWorkflows: map[string]valid.Workflow{
					"default": defaultCfg.DeploymentWorkflows["default"],
				},
				Temporal:          valid.Temporal{TerraformTaskQueue: raw.DefaultTaskqueue},
				PersistenceConfig: defaultCfg.PersistenceConfig,
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := config.ParserValidator{}
			tmp, cleanup := TempDir(t)
			defer cleanup()
			path := filepath.Join(tmp, "conf.yaml")
			Ok(t, os.WriteFile(path, []byte(c.input), 0600))

			act, err := r.ParseGlobalCfg(path, valid.NewGlobalCfg("somedir"))

			if c.expErr != "" {
				expErr := strings.ReplaceAll(c.expErr, "<tmp>", path)
				ErrEquals(t, expErr, err)
				return
			}
			Ok(t, err)

			if !act.PolicySets.HasPolicies() {
				c.exp.PolicySets = act.PolicySets
			}

			Equals(t, c.exp, act)
			// Have to hand-compare regexes because Equals doesn't do it.
			for i, actRepo := range act.Repos {
				expRepo := c.exp.Repos[i]
				if expRepo.IDRegex != nil {
					Assert(t, expRepo.IDRegex.String() == actRepo.IDRegex.String(),
						"%q != %q for repos[%d]", expRepo.IDRegex.String(), actRepo.IDRegex.String(), i)
				}
				if expRepo.BranchRegex != nil {
					Assert(t, expRepo.BranchRegex.String() == actRepo.BranchRegex.String(),
						"%q != %q for repos[%d]", expRepo.BranchRegex.String(), actRepo.BranchRegex.String(), i)
				}
			}
		})
	}
}

func TestParseGlobalCfg_PlatformMode(t *testing.T) {
	defaultCfg := valid.NewGlobalCfg("somedir")
	preWorkflowHook := &valid.PreWorkflowHook{
		StepName:   "run",
		RunCommand: "custom workflow command",
	}
	preWorkflowHooks := []*valid.PreWorkflowHook{preWorkflowHook}

	customPlan1 := valid.Stage{
		Steps: []valid.Step{
			{
				StepName:   "run",
				RunCommand: "custom command",
			},
			{
				StepName:  "init",
				ExtraArgs: []string{"extra", "args"},
			},
			{
				StepName: "plan",
			},
		},
	}
	customPolicyCheck1 := valid.Stage{
		Steps: []valid.Step{
			{
				StepName:   "run",
				RunCommand: "custom command",
			},
			{
				StepName:  "plan",
				ExtraArgs: []string{"extra", "args"},
			},
			{
				StepName: "policy_check",
			},
		},
	}
	customApply1 := valid.Stage{
		Steps: []valid.Step{
			{
				StepName:   "run",
				RunCommand: "custom command",
			},
			{
				StepName: "apply",
			},
		},
	}

	customPulRequestWorkflow1 := valid.Workflow{
		Name:        "custom1",
		Plan:        customPlan1,
		PolicyCheck: customPolicyCheck1,
	}

	customDeploymentWorkflow1 := valid.Workflow{
		Name:  "custom1",
		Plan:  customPlan1,
		Apply: customApply1,
	}

	conftestVersion, _ := version.NewVersion("v1.0.0")

	cases := map[string]struct {
		input  string
		expErr string
		exp    valid.GlobalCfg
	}{
		"pull_request_workflows don't support apply": {
			input: `
pull_request_workflows:
  default:
    apply:
      steps:
        - run: custom
`,
			expErr: "yaml: unmarshal errors:\n  line 4: field apply not found in type raw.PullRequestWorkflow",
		},
		"deployment_workflows don't support policy_checks": {
			input: `
deployment_workflows:
  default:
    policy_check:
      steps:
        - run: custom
`,
			expErr: "yaml: unmarshal errors:\n  line 4: field policy_check not found in type raw.DeploymentWorkflow",
		},
		"all keys specified": {
			input: `
repos:
- id: github.com/owner/repo
  pre_workflow_hooks:
    - run: custom workflow command
  pull_request_workflow: custom1
  deployment_workflow: custom1
  allowed_overrides: [apply_requirements, pull_request_workflow, deployment_workflow, workflow]
- id: /.*/
  branch: /(master|main)/
  pre_workflow_hooks:
    - run: custom workflow command
pull_request_workflows:
  custom1:
    plan:
      steps:
      - run: custom command
      - init:
          extra_args: [extra, args]
      - plan
    policy_check:
      steps:
      - run: custom command
      - plan:
          extra_args: [extra, args]
      - policy_check
deployment_workflows:
  custom1:
    plan:
      steps:
      - run: custom command
      - init:
          extra_args: [extra, args]
      - plan
    apply:
      steps:
      - run: custom command
      - apply
policies:
  conftest_version: v1.0.0
  policy_sets:
    - name: good-policy
      paths: [rel/path/to/policy]
`,
			exp: valid.GlobalCfg{
				Repos: []valid.Repo{
					defaultCfg.Repos[0],
					{
						ID:                  "github.com/owner/repo",
						PreWorkflowHooks:    preWorkflowHooks,
						PullRequestWorkflow: &customPulRequestWorkflow1,
						DeploymentWorkflow:  &customDeploymentWorkflow1,
						ApplyRequirements:   []string{"policies_passed"},
						AllowedOverrides:    []string{"apply_requirements", "pull_request_workflow", "deployment_workflow", "workflow"},
						CheckoutStrategy:    "branch",
						ApplySettings: valid.ApplySettings{
							BranchRestriction: valid.DefaultBranchRestriction,
						},
					},
					{
						IDRegex:           regexp.MustCompile(".*"),
						BranchRegex:       regexp.MustCompile("(master|main)"),
						ApplyRequirements: []string{"policies_passed"},
						PreWorkflowHooks:  preWorkflowHooks,
						CheckoutStrategy:  "branch",
						ApplySettings: valid.ApplySettings{
							BranchRestriction: valid.DefaultBranchRestriction,
						},
					},
				},
				PullRequestWorkflows: map[string]valid.Workflow{
					"default": defaultCfg.PullRequestWorkflows["default"],
					"custom1": customPulRequestWorkflow1,
				},
				DeploymentWorkflows: map[string]valid.Workflow{
					"default": defaultCfg.DeploymentWorkflows["default"],
					"custom1": customDeploymentWorkflow1,
				},
				Temporal: valid.Temporal{TerraformTaskQueue: raw.DefaultTaskqueue},
				PolicySets: valid.PolicySets{
					Version: conftestVersion,
					PolicySets: []valid.PolicySet{
						{
							Name:  "good-policy",
							Paths: []string{"rel/path/to/policy"},
						},
					},
				},
				PersistenceConfig: defaultCfg.PersistenceConfig,
			},
		},
		"redefine default platform workflows": {
			input: `
pull_request_workflows:
  default:
    plan:
      steps:
      - run: custom
    policy_check:
      steps: []
deployment_workflows:
  default:
    plan:
      steps: []
    apply:
      steps:
      - run: custom
`,
			exp: valid.GlobalCfg{
				Repos: []valid.Repo{
					{
						IDRegex:           regexp.MustCompile(".*"),
						BranchRegex:       regexp.MustCompile(".*"),
						ApplyRequirements: []string{},
						PullRequestWorkflow: &valid.Workflow{
							Name: "default",
							Apply: valid.Stage{
								Steps: nil,
							},
							PolicyCheck: valid.Stage{
								Steps: nil,
							},
							Plan: valid.Stage{
								Steps: []valid.Step{
									{
										StepName:   "run",
										RunCommand: "custom",
									},
								},
							},
						},
						DeploymentWorkflow: &valid.Workflow{
							Name: "default",
							Apply: valid.Stage{
								Steps: []valid.Step{
									{
										StepName:   "run",
										RunCommand: "custom",
									},
								},
							},
							PolicyCheck: valid.Stage{
								Steps: nil,
							},
							Plan: valid.Stage{
								Steps: nil,
							},
						},
						AllowedWorkflows: []string{},
						AllowedOverrides: []string{},
						CheckoutStrategy: "branch",
						ApplySettings: valid.ApplySettings{
							BranchRestriction: valid.DefaultBranchRestriction,
						},
					},
				},
				PullRequestWorkflows: map[string]valid.Workflow{
					"default": {
						Name: "default",
						Plan: valid.Stage{
							Steps: []valid.Step{
								{
									StepName:   "run",
									RunCommand: "custom",
								},
							},
						},
					},
				},
				DeploymentWorkflows: map[string]valid.Workflow{
					"default": {
						Name: "default",
						Apply: valid.Stage{
							Steps: []valid.Step{
								{
									StepName:   "run",
									RunCommand: "custom",
								},
							},
						},
					},
				},
				Temporal:          valid.Temporal{TerraformTaskQueue: raw.DefaultTaskqueue},
				PersistenceConfig: defaultCfg.PersistenceConfig,
			},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := &config.ParserValidator{}
			tmp, cleanup := TempDir(t)
			defer cleanup()
			path := filepath.Join(tmp, "conf.yaml")
			Ok(t, os.WriteFile(path, []byte(c.input), 0600))

			act, err := r.ParseGlobalCfg(path, valid.NewGlobalCfg("somedir"))

			if c.expErr != "" {
				expErr := strings.ReplaceAll(c.expErr, "<tmp>", path)
				ErrEquals(t, expErr, err)
				return
			}
			Ok(t, err)

			if !act.PolicySets.HasPolicies() {
				c.exp.PolicySets = act.PolicySets
			}

			Equals(t, c.exp, act)
			// Have to hand-compare regexes because Equals doesn't do it.
			for i, actRepo := range act.Repos {
				expRepo := c.exp.Repos[i]
				if expRepo.IDRegex != nil {
					Assert(t, expRepo.IDRegex.String() == actRepo.IDRegex.String(),
						"%q != %q for repos[%d]", expRepo.IDRegex.String(), actRepo.IDRegex.String(), i)
				}
				if expRepo.BranchRegex != nil {
					Assert(t, expRepo.BranchRegex.String() == actRepo.BranchRegex.String(),
						"%q != %q for repos[%d]", expRepo.BranchRegex.String(), actRepo.BranchRegex.String(), i)
				}
			}
		})
	}
}

// Test that if we pass in JSON strings everything should parse fine.
func TestParserValidator_ParseGlobalCfgJSON(t *testing.T) {
	customWorkflow := valid.Workflow{
		Name: "custom",
		Plan: valid.Stage{
			Steps: []valid.Step{
				{
					StepName: "init",
				},
				{
					StepName:  "plan",
					ExtraArgs: []string{"extra", "args"},
				},
				{
					StepName:   "run",
					RunCommand: "custom plan",
				},
			},
		},
		PolicyCheck: valid.Stage{
			Steps: []valid.Step{
				{
					StepName: "plan",
				},
				{
					StepName:   "run",
					RunCommand: "custom policy_check",
				},
			},
		},
	}

	conftestVersion, _ := version.NewVersion("v1.0.0")
	gCfg := valid.NewGlobalCfg("somedir")
	gCfg.Temporal.TerraformTaskQueue = raw.DefaultTaskqueue

	cases := map[string]struct {
		json   string
		exp    valid.GlobalCfg
		expErr string
	}{
		"empty string": {
			json:   "",
			expErr: "unexpected end of JSON input",
		},
		"empty object": {
			json: "{}",
			exp:  gCfg,
		},
		"setting all keys": {
			json: `
{
  "repos": [
    {
      "id": "/.*/",
      "apply_requirements": ["mergeable", "approved"],
      "allowed_overrides": ["pull_request_workflow", "apply_requirements"],
      "allow_custom_workflows": true
    },
    {
      "id": "github.com/owner/repo"
    }
  ],
  "pull_request_workflows": {
    "custom": {
      "plan": {
        "steps": [
          "init",
          {"plan": {"extra_args": ["extra", "args"]}},
          {"run": "custom plan"}
        ]
      },
      "policy_check": {
        "steps": [
          "plan",
          {"run": "custom policy_check"}
        ]
      }
    }
  },
  "policies": {
    "conftest_version": "v1.0.0",
    "policy_sets": [
      {
        "name": "good-policy",
        "paths": ["rel/path/to/policy"]
      }
    ]
  }
}
`,
			exp: valid.GlobalCfg{
				Repos: []valid.Repo{
					valid.NewGlobalCfg("somedir").Repos[0],
					{
						IDRegex:           regexp.MustCompile(".*"),
						ApplyRequirements: []string{"mergeable", "approved", "policies_passed"},
						AllowedOverrides:  []string{"pull_request_workflow", "apply_requirements"},
						CheckoutStrategy:  "branch",
						ApplySettings: valid.ApplySettings{
							BranchRestriction: valid.DefaultBranchRestriction,
						},
					},
					{
						ID:                "github.com/owner/repo",
						IDRegex:           nil,
						ApplyRequirements: []string{"policies_passed"},
						AllowedOverrides:  nil,
						CheckoutStrategy:  "branch",
						ApplySettings: valid.ApplySettings{
							BranchRestriction: valid.DefaultBranchRestriction,
						},
					},
				},
				PullRequestWorkflows: map[string]valid.Workflow{
					"default": valid.NewGlobalCfg("somedir").PullRequestWorkflows["default"],
					"custom":  customWorkflow,
				},
				DeploymentWorkflows: map[string]valid.Workflow{
					"default": valid.NewGlobalCfg("somedir").DeploymentWorkflows["default"],
				},
				Temporal: valid.Temporal{TerraformTaskQueue: raw.DefaultTaskqueue},
				PolicySets: valid.PolicySets{
					Version: conftestVersion,
					PolicySets: []valid.PolicySet{
						{
							Name:  "good-policy",
							Paths: []string{"rel/path/to/policy"},
						},
					},
				},
				PersistenceConfig: gCfg.PersistenceConfig,
			},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			pv := &config.ParserValidator{}
			cfg, err := pv.ParseGlobalCfgJSON(c.json, valid.NewGlobalCfg("somedir"))
			if c.expErr != "" {
				ErrEquals(t, c.expErr, err)
				return
			}
			Ok(t, err)

			if !cfg.PolicySets.HasPolicies() {
				c.exp.PolicySets = cfg.PolicySets
			}

			Equals(t, c.exp, cfg)
		})
	}
}

// Test that if we pass in JSON strings everything should parse fine.
func TestParserValidator_ParseGlobalCfgV2JSON(t *testing.T) {
	customPullRequestWorkflow := valid.Workflow{
		Name: "custom",
		Plan: valid.Stage{
			Steps: []valid.Step{
				{
					StepName: "init",
				},
				{
					StepName:  "plan",
					ExtraArgs: []string{"extra", "args"},
				},
				{
					StepName:   "run",
					RunCommand: "custom plan",
				},
			},
		},
		PolicyCheck: valid.Stage{
			Steps: []valid.Step{
				{
					StepName: "plan",
				},
				{
					StepName:   "run",
					RunCommand: "custom policy_check",
				},
			},
		},
	}

	customDeploymentWorkflow := valid.Workflow{
		Name: "custom",
		Plan: valid.Stage{
			Steps: []valid.Step{
				{
					StepName: "init",
				},
				{
					StepName:  "plan",
					ExtraArgs: []string{"extra", "args"},
				},
				{
					StepName:   "run",
					RunCommand: "custom plan",
				},
			},
		},
		Apply: valid.Stage{
			Steps: []valid.Step{
				{
					StepName:   "run",
					RunCommand: "my custom command",
				},
			},
		},
	}

	conftestVersion, _ := version.NewVersion("v1.0.0")
	globalCfg := valid.NewGlobalCfg("somedir")
	globalCfg.Temporal.TerraformTaskQueue = raw.DefaultTaskqueue

	cases := map[string]struct {
		json   string
		exp    valid.GlobalCfg
		expErr string
	}{
		"empty string": {
			json:   "",
			expErr: "unexpected end of JSON input",
		},
		"empty object": {
			json: "{}",
			exp:  globalCfg,
		},
		"setting all keys": {
			json: `
{
  "repos": [
    {
      "id": "/.*/",
      "pull_request_workflow": "custom",
      "deployment_workflow": "custom",
      "allowed_pull_request_workflows": ["custom"],
      "allowed_deployment_workflows": ["custom"],
      "allowed_overrides": ["pull_request_workflow", "deployment_workflow"],
      "allow_custom_workflows": true
    },
    {
      "id": "github.com/owner/repo"
    }
  ],
  "pull_request_workflows": {
    "custom": {
      "plan": {
        "steps": [
          "init",
          {"plan": {"extra_args": ["extra", "args"]}},
          {"run": "custom plan"}
        ]
      },
      "policy_check": {
        "steps": [
          "plan",
          {"run": "custom policy_check"}
        ]
      }
    }
  },
  "deployment_workflows": {
    "custom": {
      "plan": {
        "steps": [
          "init",
          {"plan": {"extra_args": ["extra", "args"]}},
          {"run": "custom plan"}
        ]
      },
      "apply": {
        "steps": [
          {"run": "my custom command"}
        ]
      }
    }
  },
  "policies": {
    "conftest_version": "v1.0.0",
    "policy_sets": [
      {
        "name": "good-policy",
        "paths": ["rel/path/to/policy"]
      }
    ]
  }
}
`,
			exp: valid.GlobalCfg{
				Repos: []valid.Repo{
					globalCfg.Repos[0],
					{
						IDRegex:                     regexp.MustCompile(".*"),
						ApplyRequirements:           []string{"policies_passed"},
						PullRequestWorkflow:         &customPullRequestWorkflow,
						DeploymentWorkflow:          &customDeploymentWorkflow,
						AllowedPullRequestWorkflows: []string{"custom"},
						AllowedDeploymentWorkflows:  []string{"custom"},
						AllowedOverrides:            []string{"pull_request_workflow", "deployment_workflow"},
						CheckoutStrategy:            "branch",
						ApplySettings: valid.ApplySettings{
							BranchRestriction: valid.DefaultBranchRestriction,
						},
					},
					{
						ID:                          "github.com/owner/repo",
						IDRegex:                     nil,
						ApplyRequirements:           []string{"policies_passed"},
						AllowedOverrides:            nil,
						AllowedPullRequestWorkflows: nil,
						AllowedDeploymentWorkflows:  nil,
						CheckoutStrategy:            "branch",
						ApplySettings: valid.ApplySettings{
							BranchRestriction: valid.DefaultBranchRestriction,
						},
					},
				},
				PullRequestWorkflows: map[string]valid.Workflow{
					"default": globalCfg.PullRequestWorkflows["default"],
					"custom":  customPullRequestWorkflow,
				},
				DeploymentWorkflows: map[string]valid.Workflow{
					"default": globalCfg.DeploymentWorkflows["default"],
					"custom":  customDeploymentWorkflow,
				},
				Temporal: valid.Temporal{TerraformTaskQueue: raw.DefaultTaskqueue},
				PolicySets: valid.PolicySets{
					Version: conftestVersion,
					PolicySets: []valid.PolicySet{
						{
							Name:  "good-policy",
							Paths: []string{"rel/path/to/policy"},
						},
					},
				},
				PersistenceConfig: globalCfg.PersistenceConfig,
			},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			pv := &config.ParserValidator{}

			cfg, err := pv.ParseGlobalCfgJSON(c.json, globalCfg)
			if c.expErr != "" {
				ErrEquals(t, c.expErr, err)
				return
			}
			Ok(t, err)

			if !cfg.PolicySets.HasPolicies() {
				c.exp.PolicySets = cfg.PolicySets
			}

			Equals(t, c.exp, cfg)
		})
	}
}
