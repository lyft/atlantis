package valid

import (
	"github.com/hashicorp/go-version"
)

const (
	LocalPolicySet  string = "local"
	GithubPolicySet string = "github"
)

// PolicySets defines version of policy checker binary(conftest) and a list of
// PolicySet objects. PolicySets struct is used by PolicyCheck workflow to build
// context to enforce policies.
type PolicySets struct {
	Version      *version.Version
	PolicySets   []PolicySet
	Organization string // Github organization each policy set owner belongs to
}

type PolicyOwners struct {
	Users []string
}

type PolicySet struct {
	Name  string
	Owner string
	Paths []string
}

func (p *PolicySets) HasPolicies() bool {
	return len(p.PolicySets) > 0
}
