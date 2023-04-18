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
