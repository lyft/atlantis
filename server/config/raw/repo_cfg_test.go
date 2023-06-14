package raw_test

import (
	"testing"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/runatlantis/atlantis/server/config/raw"
	. "github.com/runatlantis/atlantis/testing"
	yaml "gopkg.in/yaml.v2"
)

func TestConfig_UnmarshalYAML(t *testing.T) {
	cases := []struct {
		description string
		input       string
		exp         raw.RepoCfg
		expErr      string
	}{
		{
			description: "no data",
			input:       "",
			exp: raw.RepoCfg{
				Version:  nil,
				Projects: nil,
			},
		},
		{
			description: "yaml nil",
			input:       "~",
			exp: raw.RepoCfg{
				Version:  nil,
				Projects: nil,
			},
		},
		{
			description: "invalid key",
			input:       "invalid: key",
			exp: raw.RepoCfg{
				Version:  nil,
				Projects: nil,
			},
			expErr: "yaml: unmarshal errors:\n  line 1: field invalid not found in type raw.RepoCfg",
		},
		{
			description: "version set to 2",
			input:       "version: 2",
			exp: raw.RepoCfg{
				Version:  Int(2),
				Projects: nil,
			},
		},
		{
			description: "version set to 3",
			input:       "version: 3",
			exp: raw.RepoCfg{
				Version:  Int(3),
				Projects: nil,
			},
		},
		{
			description: "projects key without value",
			input:       "projects:",
			exp: raw.RepoCfg{
				Version:  nil,
				Projects: nil,
			},
		},
		{
			description: "projects with a map",
			input:       "projects:\n  key: value",
			exp: raw.RepoCfg{
				Version:  nil,
				Projects: nil,
			},
			expErr: "yaml: unmarshal errors:\n  line 2: cannot unmarshal !!map into []raw.Project",
		},
		{
			description: "projects with a scalar",
			input:       "projects: value",
			exp: raw.RepoCfg{
				Version:  nil,
				Projects: nil,
			},
			expErr: "yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `value` into []raw.Project",
		},
		{
			description: "parallel apply not a boolean",
			input:       "version: 3\nparallel_apply: notabool",
			exp: raw.RepoCfg{
				Version:  nil,
				Projects: nil,
			},
			expErr: "yaml: unmarshal errors:\n  line 2: cannot unmarshal !!str `notabool` into bool",
		},
		{
			description: "should use values if set",
			input: `
version: 3
parallel_apply: true
parallel_plan: false
projects:
- dir: mydir
  workspace: myworkspace
  workflow: default
  terraform_version: v0.11.0
  autoplan:
    enabled: false
    when_modified: []
  apply_requirements: [mergeable]`,
			exp: raw.RepoCfg{
				Version:       Int(3),
				ParallelApply: Bool(true),
				ParallelPlan:  Bool(false),
				Projects: []raw.Project{
					{
						Dir:              String("mydir"),
						Workspace:        String("myworkspace"),
						Workflow:         String("default"),
						TerraformVersion: String("v0.11.0"),
						Autoplan: &raw.Autoplan{
							WhenModified: []string{},
							Enabled:      Bool(false),
						},
						ApplyRequirements: []string{"mergeable"},
					},
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			var conf raw.RepoCfg
			err := yaml.UnmarshalStrict([]byte(c.input), &conf)
			if c.expErr != "" {
				ErrEquals(t, c.expErr, err)
				return
			}
			Ok(t, err)
			Equals(t, c.exp, conf)
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	cases := []struct {
		description string
		input       raw.RepoCfg
		expErr      string
	}{
		{
			description: "version not nil",
			input: raw.RepoCfg{
				Version: nil,
			},
			expErr: "version: is required. If you've just upgraded Atlantis you need to rewrite your atlantis.yaml for version 3. See www.runatlantis.io/docs/upgrading-atlantis-yaml.html.",
		},
		{
			description: "version not 2 or 3",
			input: raw.RepoCfg{
				Version: Int(1),
			},
			expErr: "version: only versions 2 and 3 are supported.",
		},
	}
	validation.ErrorTag = "yaml"
	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			err := c.input.Validate()
			if c.expErr == "" {
				Ok(t, err)
			} else {
				ErrEquals(t, c.expErr, err)
			}
		})
	}
}
