package raw

import (
	"errors"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/runatlantis/atlantis/server/config/valid"
)

// DefaultParallelApply is the default setting for parallel apply
const DefaultParallelApply = false

// DefaultParallelPlan is the default setting for parallel plan
const DefaultParallelPlan = false

// DefaultParallelPolicyCheck is the default setting for parallel plan
const DefaultParallelPolicyCheck = false

// RepoCfg is the raw schema for repo-level atlantis.yaml config.
type RepoCfg struct {
	Version       *int       `yaml:"version,omitempty"`
	Projects      []Project  `yaml:"projects,omitempty"`
	PolicySets    PolicySets `yaml:"policies,omitempty"`
	ParallelApply *bool      `yaml:"parallel_apply,omitempty"`
	ParallelPlan  *bool      `yaml:"parallel_plan,omitempty"`
	// Deprecated
	WorkflowModeType string `yaml:"workflow_mode_type,omitempty"`
}

func (r RepoCfg) Validate() error {
	equals2 := func(value interface{}) error {
		asIntPtr := value.(*int)
		if asIntPtr == nil {
			return errors.New("is required. If you've just upgraded Atlantis you need to rewrite your atlantis.yaml for version 3. See www.runatlantis.io/docs/upgrading-atlantis-yaml.html")
		}
		if *asIntPtr != 2 && *asIntPtr != 3 {
			return errors.New("only versions 2 and 3 are supported")
		}
		return nil
	}
	return validation.ValidateStruct(&r,
		validation.Field(&r.Version, validation.By(equals2)),
		validation.Field(&r.Projects),
		validation.Field(&r.WorkflowModeType, validation.In("platform")),
	)
}

func (r RepoCfg) ToValid() valid.RepoCfg {
	workflowModeType := valid.PlatformWorkflowMode

	var validProjects []valid.Project
	for _, p := range r.Projects {
		validProjects = append(validProjects, p.ToValid(workflowModeType))
	}

	parallelApply := DefaultParallelApply
	if r.ParallelApply != nil {
		parallelApply = *r.ParallelApply
	}

	parallelPlan := DefaultParallelPlan
	if r.ParallelPlan != nil {
		parallelPlan = *r.ParallelPlan
	}

	return valid.RepoCfg{
		Version:             *r.Version,
		Projects:            validProjects,
		ParallelApply:       parallelApply,
		ParallelPlan:        parallelPlan,
		ParallelPolicyCheck: parallelPlan,
	}
}
