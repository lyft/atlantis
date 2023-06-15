package requirement

import (
	"context"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/models"
	"github.com/runatlantis/atlantis/server/neptune/template"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
)

type Criteria struct {
	User              models.User
	Branch            string
	Repo              models.Repo
	OptionalPull      *models.PullRequest
	InstallationToken int64
	TriggerInfo       workflows.DeployTriggerInfo
	Roots             []*valid.MergedProjectCfg
}

type fetcher interface {
	ListTeamMembers(ctx context.Context, installationToken int64, teamSlug string) ([]string, error)
}

type team struct {
	cfg            valid.GlobalCfg
	fetcher        fetcher
	errorGenerator errGenerator[template.UserForbiddenData]
}

func (r *team) Check(ctx context.Context, criteria Criteria) error {
	match := r.cfg.MatchingRepo(criteria.Repo.ID())

	if len(match.ApplySettings.Team) == 0 {
		return nil
	}

	teamMembers, err := r.fetcher.ListTeamMembers(ctx, criteria.InstallationToken, match.ApplySettings.Team)
	if err != nil {
		return errors.Wrap(err, "fetching team members")
	}

	for _, t := range teamMembers {
		if criteria.User.Username == t {
			return nil
		}
	}

	return r.errorGenerator.GenerateForbiddenError(
		ctx,
		template.UserForbidden, criteria.Repo,
		template.UserForbiddenData{
			User: criteria.User.Username,
			Team: match.ApplySettings.Team,
			Org:  r.cfg.PolicySets.Organization,
		},
		"User: %s is forbidden from executing a deploy", criteria.User.Username,
	)
}
