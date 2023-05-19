package requirement

import (
	"context"

	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/neptune/template"
	"github.com/runatlantis/atlantis/server/vcs/provider/github"
)

type Requirement interface {
	Check(ctx context.Context, criteria Criteria) error
}
type DeployAggregate struct {
	overrideableRequirements    []Requirement
	nonOverrideableRequirements []Requirement
}

func NewDeployAggregateWithRequirements(overrideableRequirements []Requirement, nonOverrideableRequirements []Requirement) *DeployAggregate {
	return &DeployAggregate{
		overrideableRequirements:    overrideableRequirements,
		nonOverrideableRequirements: nonOverrideableRequirements,
	}
}

func NewDeployAggregate(cfg valid.GlobalCfg, teamFetcher *github.TeamMemberFetcher, reviewFetcher *github.PRReviewFetcher, checkRunFetcher *github.CheckRunsFetcher, logger logging.Logger) *DeployAggregate {
	return NewDeployAggregateWithRequirements(

		// overrideable
		[]Requirement{

			// order matters here since we fail iteratively
			&branchRestriction{
				cfg: cfg,
				errorGenerator: errorGenerator[template.BranchForbiddenData]{
					logger: logger,
					loader: template.Loader[template.BranchForbiddenData]{GlobalCfg: cfg},
				},
			},
			&team{
				cfg:     cfg,
				fetcher: teamFetcher,
				errorGenerator: errorGenerator[template.UserForbiddenData]{
					logger: logger,
					loader: template.Loader[template.UserForbiddenData]{GlobalCfg: cfg},
				},
			},
			&approval{
				cfg:     cfg,
				fetcher: reviewFetcher,
				errorGenerator: errorGenerator[template.Input]{
					logger: logger,
					loader: template.Loader[template.Input]{GlobalCfg: cfg},
				},
			},
			&planValidationResult{
				cfg: cfg,
				errorGenerator: errorGenerator[template.PlanValidationSuccessData]{
					logger: logger,
					loader: template.Loader[template.PlanValidationSuccessData]{GlobalCfg: cfg},
				},
				fetcher: checkRunFetcher,
			},
		},

		// non-overrideable
		[]Requirement{
			pull{},
		},
	)
}

func (a *DeployAggregate) Check(ctx context.Context, criteria Criteria) error {
	for _, d := range a.nonOverrideableRequirements {
		if err := d.Check(ctx, criteria); err != nil {
			return err
		}
	}

	// bypass all overrideable requirements if we are forcing the deployment
	if criteria.TriggerInfo.Force {
		return nil
	}

	for _, d := range a.overrideableRequirements {
		if err := d.Check(ctx, criteria); err != nil {
			return err
		}
	}
	return nil
}

type PRAggregate struct {
	requirements []Requirement
}

func NewPRAggregate(globalCfg valid.GlobalCfg) *PRAggregate {
	return &PRAggregate{
		requirements: []Requirement{
			pull{},
			baseBranch{
				GlobalCfg: globalCfg,
			},
		},
	}
}

func (p *PRAggregate) Check(ctx context.Context, criteria Criteria) error {
	for _, r := range p.requirements {
		if err := r.Check(ctx, criteria); err != nil {
			return err
		}
	}
	return nil
}
