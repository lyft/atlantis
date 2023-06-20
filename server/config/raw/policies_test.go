package raw_test

import (
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/config/raw"
	"github.com/runatlantis/atlantis/server/config/valid"
	. "github.com/runatlantis/atlantis/testing"
	yaml "gopkg.in/yaml.v2"
)

func TestPolicySetsConfig_YAMLMarshalling(t *testing.T) {
	cases := []struct {
		description string
		input       string
		exp         raw.PolicySets
		expErr      string
	}{
		{
			description: "valid yaml",
			input: `
conftest_version: v1.0.0
organization: org
policy_sets:
- name: policy-name
  paths: ["rel/path/to/policy-set", "rel/path/to/another/policy-set"]
`,
			exp: raw.PolicySets{
				Organization: "org",
				Version:      String("v1.0.0"),
				PolicySets: []raw.PolicySet{
					{
						Name:  "policy-name",
						Paths: []string{"rel/path/to/policy-set", "rel/path/to/another/policy-set"},
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			var got raw.PolicySets
			err := yaml.UnmarshalStrict([]byte(c.input), &got)
			if c.expErr != "" {
				ErrEquals(t, c.expErr, err)
				return
			}
			Ok(t, err)
			Equals(t, c.exp, got)

			_, err = yaml.Marshal(got)
			Ok(t, err)

			var got2 raw.PolicySets
			err = yaml.UnmarshalStrict([]byte(c.input), &got2)
			Ok(t, err)
			Equals(t, got2, got)
		})
	}
}

func TestPolicySets_Validate(t *testing.T) {
	cases := []struct {
		description string
		input       raw.PolicySets
		expErr      string
	}{
		// Valid inputs.
		{
			description: "policies",
			input: raw.PolicySets{
				Organization: "org",
				Version:      String("v1.0.0"),
				PolicySets: []raw.PolicySet{
					{
						Name:  "policy-name-1",
						Owner: "owner1",
						Paths: []string{"rel/path/to/source"},
					},
					{
						Name:  "policy-name-2",
						Owner: "owner2",
						Paths: []string{"rel/path/to/source"},
					},
					{
						Name:  "policy-name-3",
						Owner: "owner3",
						Paths: []string{"rel/path/to/source", "rel/diff/path/to/source"},
					},
				},
			},
			expErr: "",
		},

		// Invalid inputs.
		{
			description: "empty elem",
			input:       raw.PolicySets{},
			expErr:      "policy_sets: cannot be empty; Declare policies that you would like to enforce.",
		},

		{
			description: "missing policy name",
			input: raw.PolicySets{
				PolicySets: []raw.PolicySet{
					{},
				},
			},
			expErr: "policy_sets: (0: (name: is required; owner: is required; paths: is required.).).",
		},
		{
			description: "empty string version",
			input: raw.PolicySets{
				Version: String(""),
				PolicySets: []raw.PolicySet{
					{
						Name:  "policy-name-1",
						Owner: "owner1",
						Paths: []string{"rel/path/to/source"},
					},
				},
			},
			expErr: "conftest_version: version \"\" could not be parsed: Malformed version: .",
		},
		{
			description: "invalid version",
			input: raw.PolicySets{
				Version: String("version123"),
				PolicySets: []raw.PolicySet{
					{
						Name:  "policy-name-1",
						Owner: "owner1",
						Paths: []string{"rel/path/to/source"},
					},
				},
			},
			expErr: "conftest_version: version \"version123\" could not be parsed: Malformed version: version123.",
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			err := c.input.Validate()
			if c.expErr == "" {
				Ok(t, err)
				return
			}
			ErrEquals(t, c.expErr, err)
		})
	}
}

func TestPolicySets_ToValid(t *testing.T) {
	version, _ := version.NewVersion("v1.0.0")
	cases := []struct {
		description string
		input       raw.PolicySets
		exp         valid.PolicySets
	}{
		{
			description: "valid policies",
			input: raw.PolicySets{
				Organization: "org",
				Version:      String("v1.0.0"),
				PolicySets: []raw.PolicySet{
					{
						Name:  "good-policy",
						Paths: []string{"rel/path/to/source"},
					},
				},
			},
			exp: valid.PolicySets{
				Organization: "org",
				Version:      version,
				PolicySets: []valid.PolicySet{
					{
						Name:  "good-policy",
						Paths: []string{"rel/path/to/source"},
					},
				},
			},
		},
		{
			description: "valid policies with multiple paths",
			input: raw.PolicySets{
				Organization: "org",
				Version:      String("v1.0.0"),
				PolicySets: []raw.PolicySet{
					{
						Name:  "good-policy",
						Paths: []string{"rel/path/to/source", "rel/path/to/source2"},
					},
				},
			},
			exp: valid.PolicySets{
				Organization: "org",
				Version:      version,
				PolicySets: []valid.PolicySet{
					{
						Name:  "good-policy",
						Paths: []string{"rel/path/to/source", "rel/path/to/source2"},
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			Equals(t, c.exp, c.input.ToValid())
		})
	}
}
