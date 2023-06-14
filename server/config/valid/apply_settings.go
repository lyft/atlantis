package valid

type BranchRestriction string

const (
	NoBranchRestriction      BranchRestriction = "none"
	DefaultBranchRestriction BranchRestriction = "default_branch"
)

type ApplySettings struct {
	PRRequirements    []string
	BranchRestriction BranchRestriction
	Team              string
}

func (s ApplySettings) ContainsPRRequirement(req string) bool {
	for _, r := range s.PRRequirements {
		if r == req {
			return true
		}
	}
	return false
}
