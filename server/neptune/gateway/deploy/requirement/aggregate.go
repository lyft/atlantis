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
	delegates []requirement
}

func NewAggregate(cfg valid.GlobalCfg, fetcher *github.TeamMemberFetcher) *Aggregate {
	delegates := []requirement{
		&team{
			cfg:     cfg,
			fetcher: fetcher,
		},
		&branchRestriction{
			cfg: cfg,
		},
	}

	return &Aggregate{
		delegates: delegates,
	}
}

func (a *Aggregate) Check(ctx context.Context, criteria Criteria) error {
	// bypass all requirements if we are forcing the deployment to happen
	if criteria.TriggerInfo.Force {
		return nil
	}

	for _, d := range a.delegates {
		if err := d.Check(ctx, criteria); err != nil {
			return err
		}
	}
	return nil
}
