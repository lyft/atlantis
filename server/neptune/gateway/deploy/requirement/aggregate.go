package requirement

import (
	"context"

	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/vcs/provider/github"
)

type requirement interface {
	Check(ctx context.Context, criteria Criteria) error
}
type Aggregate struct {
	overrideableRequirements    []requirement
	nonOverrideableRequirements []requirement
}

func NewAggregate(cfg valid.GlobalCfg, fetcher *github.TeamMemberFetcher) *Aggregate {
	return &Aggregate{
		overrideableRequirements: []requirement{
			&team{
				cfg:     cfg,
				fetcher: fetcher,
			},
			&branchRestriction{
				cfg: cfg,
			},
		},
		nonOverrideableRequirements: []requirement{
			pull{},
		},
	}
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
