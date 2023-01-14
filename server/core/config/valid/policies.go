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
	Version    *version.Version
	Owners     PolicyOwners
	PolicySets []PolicySet
}

type PolicyOwners struct {
	Users []string
}

type PolicySet struct {
	Source string // TODO: seems unused, remove when legacy policy checks are deprecated
	Path   string // TODO: replaced by Paths, remove when legacy policy checks are deprecated
	Name   string
	Owner  string
	Paths  []string
}

func (p *PolicySets) HasPolicies() bool {
	return len(p.PolicySets) > 0
}

func (p *PolicySets) IsOwner(username string) bool {
	for _, uname := range p.Owners.Users {
		if uname == username {
			return true
		}
	}

	return false
}
