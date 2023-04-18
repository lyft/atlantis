package raw

import (
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/runatlantis/atlantis/server/core/config/valid"
)

type ApplySettings struct {
	PRRequirements    []string
	BranchRestriction string
	Team              string
}

func (s ApplySettings) Validate() error {
	return validation.ValidateStruct(&s,
		validation.Field(&s.PRRequirements, validation.In(valid.ApprovedApplyReq, valid.PoliciesPassedApplyReq)),
		validation.Field(&s.BranchRestriction, validation.In(valid.NoBranchRestriction, valid.DefaultBranchRestriction)),
		validation.Field(&s.Team),
	)
}

func (s ApplySettings) ToValid() valid.ApplySettings {
	branchRestriction := valid.NoBranchRestriction
	if len(s.BranchRestriction) != 0 {
		branchRestriction = valid.BranchRestriction(s.BranchRestriction)
	}

	return valid.ApplySettings{
		PRRequirements:    s.PRRequirements,
		BranchRestriction: branchRestriction,
		Team:              s.Team,
	}
}
