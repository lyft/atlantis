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
type Aggregate struct {
	overrideableRequirements    []Requirement
	nonOverrideableRequirements []Requirement
}

func NewAggregateWithRequirements(overrideableRequirements []Requirement, nonOverrideableRequirements []Requirement) *Aggregate {
	return &Aggregate{
		overrideableRequirements:    overrideableRequirements,
		nonOverrideableRequirements: nonOverrideableRequirements,
	}
}

func NewAggregate(cfg valid.GlobalCfg, teamFetcher *github.TeamMemberFetcher, reviewFetcher *github.PRReviewFetcher, logger logging.Logger) *Aggregate {
	return NewAggregateWithRequirements(

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
		},

		// non-overrideable
		[]Requirement{
			pull{},
		},
	)
}

func NewPRAggregate(globalCfg valid.GlobalCfg) *Aggregate {
	return NewAggregateWithRequirements(
		// overrideable
		[]Requirement{},
		// non-overrideable
		[]Requirement{
			pull{},
			baseBranch{
				GlobalCfg: globalCfg,
			},
		},
	)
}

func (a *Aggregate) Check(ctx context.Context, criteria Criteria) error {
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
