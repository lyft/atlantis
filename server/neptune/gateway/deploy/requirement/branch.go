package requirement

import (
	"context"
	"fmt"

	"github.com/runatlantis/atlantis/server/core/config/valid"
)

type ForbiddenError struct {
	message string
}

func NewForbiddenError(msg string, format ...string) ForbiddenError {
	return ForbiddenError{message: fmt.Sprintf(msg, format)}
}

func (e ForbiddenError) Error() string {
	return e.message
}

type branchRestriction struct {
	cfg valid.GlobalCfg
}

func (r *branchRestriction) Check(ctx context.Context, criteria Criteria) error {
	match := r.cfg.MatchingRepo(criteria.Repo.ID())

	if match.ApplySettings.BranchRestriction == valid.DefaultBranchRestriction && criteria.Repo.DefaultBranch != criteria.Branch {
		return NewForbiddenError("deploys are forbidden on %s branch", criteria.Branch)
	}

	return nil
}
