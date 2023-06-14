package raw

import (
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/runatlantis/atlantis/server/config/valid"
)

type ApplySettings struct {
	PRRequirements    []string `yaml:"pr_requirements" json:"pr_requirements"`
	BranchRestriction string   `yaml:"branch_restriction" json:"branch_restriction"`
	Team              string   `yaml:"team" json:"team"`
}

func (s ApplySettings) Validate() error {
	return validation.ValidateStruct(&s,
		validation.Field(&s.PRRequirements, validation.By(func(value interface{}) error {
			v := value.([]string)

			for _, item := range v {
				return validation.In(valid.ApprovedApplyReq).Validate(item)
			}
			return nil
		})),
		validation.Field(&s.BranchRestriction, validation.In(string(valid.NoBranchRestriction), string(valid.DefaultBranchRestriction))),
	)
}

func (s ApplySettings) ToValid() valid.ApplySettings {
	branchRestriction := valid.DefaultBranchRestriction
	if len(s.BranchRestriction) != 0 {
		branchRestriction = valid.BranchRestriction(s.BranchRestriction)
	}

	return valid.ApplySettings{
		PRRequirements:    s.PRRequirements,
		BranchRestriction: branchRestriction,
		Team:              s.Team,
	}
}
