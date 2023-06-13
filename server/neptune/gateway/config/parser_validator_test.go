package config_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/neptune/gateway/config"
	. "github.com/runatlantis/atlantis/testing"
	"github.com/stretchr/testify/assert"
)

var globalConfig = valid.GlobalCfg{
	Repos: []valid.Repo{
		{
			IDRegex: regexp.MustCompile(".*"),

			AllowedOverrides: []string{"apply_requirements", "pull_request_workflow"},
			CheckoutStrategy: "branch",
		},
	},
	PullRequestWorkflows: map[string]valid.Workflow{
		"myworkflow": {},
	},
}

func TestHasRepoCfg_DirDoesNotExist(t *testing.T) {
	r := config.ParserValidator{
		GlobalCfg: globalConfig,
	}
	_, err := r.ParseRepoCfg("/not/exist", "")
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestHasRepoCfg_FileDoesNotExist(t *testing.T) {
	tmpDir, cleanup := TempDir(t)
	defer cleanup()
	r := config.ParserValidator{
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

	r := config.ParserValidator{
		GlobalCfg: globalConfig,
	}
	_, err = r.ParseRepoCfg(tmpDir, "")
	assert.ErrorContains(t, err, "found \"atlantis.yml\" as config file; rename using the .yaml extension - \"atlantis.yaml\"")
}

func TestParseRepoCfg_BadPermissions(t *testing.T) {
	tmpDir, cleanup := TempDir(t)
	defer cleanup()
	err := os.WriteFile(filepath.Join(tmpDir, "atlantis.yaml"), nil, 0000)
	assert.NoError(t, err)

	r := config.ParserValidator{
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
			err := os.WriteFile(confPath, []byte(c.input), 0600)
			assert.NoError(t, err)
			r := config.ParserValidator{
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
			description: "if a plan or apply explicitly defines an empty steps key then it gets the defaults",
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
			assert.NoError(t, err)

			r := config.ParserValidator{
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
  pull_request_workflow: custom`
	err := os.WriteFile(filepath.Join(tmpDir, "atlantis.yaml"), []byte(repoCfg), 0600)
	assert.NoError(t, err)

	r := config.ParserValidator{
		GlobalCfg: valid.NewGlobalCfg("somedir"),
	}

	_, err = r.ParseRepoCfg(tmpDir, "repo_id")
	assert.ErrorContains(t, err, "repo config not allowed to set 'pull_request_workflow' key: server-side config needs 'allowed_overrides: [pull_request_workflow]'")
}
