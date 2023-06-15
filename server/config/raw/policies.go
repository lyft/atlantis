package raw

import (
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/config/valid"
)

// PolicySets is the raw schema for repo-level atlantis.yaml config.
type PolicySets struct {
	Version      *string     `yaml:"conftest_version,omitempty" json:"conftest_version,omitempty"`
	PolicySets   []PolicySet `yaml:"policy_sets" json:"policy_sets"`
	Organization string      `yaml:"organization" json:"organization"`
}

func (p PolicySets) Validate() error {
	return validation.ValidateStruct(&p,
		validation.Field(&p.Version, validation.By(VersionValidator)),
		validation.Field(&p.PolicySets, validation.Required.Error("cannot be empty; Declare policies that you would like to enforce")),
	)
}

func (p PolicySets) ToValid() valid.PolicySets {
	policySets := valid.PolicySets{}

	if p.Version != nil {
		policySets.Version, _ = version.NewVersion(*p.Version)
	}

	policySets.Organization = p.Organization

	validPolicySets := make([]valid.PolicySet, 0)
	for _, rawPolicySet := range p.PolicySets {
		validPolicySets = append(validPolicySets, rawPolicySet.ToValid())
	}
	policySets.PolicySets = validPolicySets

	return policySets
}

type PolicySet struct {
	Name  string   `yaml:"name" json:"name"`
	Owner string   `yaml:"owner,omitempty" json:"owner,omitempty"`
	Paths []string `yaml:"paths" json:"paths"`
}

func (p PolicySet) Validate() error {
	return validation.ValidateStruct(&p,
		validation.Field(&p.Name, validation.Required.Error("is required")),
		validation.Field(&p.Owner, validation.Required.Error("is required")),
		validation.Field(&p.Paths, validation.Required.Error("is required")),
	)
}

func (p PolicySet) ToValid() valid.PolicySet {
	var policySet valid.PolicySet

	policySet.Name = p.Name
	policySet.Paths = p.Paths
	policySet.Owner = p.Owner

	return policySet
}
