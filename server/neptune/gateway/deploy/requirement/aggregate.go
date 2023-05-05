package requirement

import (
	"context"

	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/neptune/template"
	"github.com/runatlantis/atlantis/server/vcs/provider/github"
)

type requirement interface {
	Check(ctx context.Context, criteria Criteria) error
}
type Aggregate struct {
	overrideableRequirements    []requirement
	nonOverrideableRequirements []requirement
}

func NewAggregate(cfg valid.GlobalCfg, fetcher *github.TeamMemberFetcher, logger logging.Logger) *Aggregate {
	return &Aggregate{
		overrideableRequirements: []requirement{

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
				fetcher: fetcher,
				errorGenerator: errorGenerator[template.UserForbiddenData]{
					logger: logger,
					loader: template.Loader[template.UserForbiddenData]{GlobalCfg: cfg},
				},
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
