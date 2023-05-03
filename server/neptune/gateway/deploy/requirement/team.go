package requirement

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"github.com/runatlantis/atlantis/server/vcs/provider/github"
)

type Criteria struct {
	User              models.User
	Branch            string
	Repo              models.Repo
	InstallationToken int64
	TriggerInfo       workflows.DeployTriggerInfo
}

type UserForbiddenError struct {
	User string
}

func (e UserForbiddenError) Error() string {
	return fmt.Sprintf("User: %s is forbidden from executing a deploy", e.User)
}

type team struct {
	cfg     valid.GlobalCfg
	fetcher *github.TeamMemberFetcher
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

	return UserForbiddenError{User: criteria.User.Username}
}
