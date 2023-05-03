package requirement

import (
	"context"
	"fmt"

	"github.com/runatlantis/atlantis/server/core/config/valid"
)

type BranchRestrictedError struct {
	Branch string
}

func (e BranchRestrictedError) Error() string {
	return fmt.Sprintf("deploys are forbidden on %s branch", e.Branch)
}

type branchRestriction struct {
	cfg valid.GlobalCfg
}

func (r *branchRestriction) Check(ctx context.Context, criteria Criteria) error {
	match := r.cfg.MatchingRepo(criteria.Repo.ID())

	if match.ApplySettings.BranchRestriction == valid.DefaultBranchRestriction && criteria.Repo.DefaultBranch != criteria.Branch {
		return BranchRestrictedError{Branch: criteria.Branch}
	}

	return nil
}
