package requirement

import (
	"context"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
)

type baseBranch struct {
	GlobalCfg valid.GlobalCfg
}

func (b baseBranch) Check(ctx context.Context, criteria Criteria) error {
	repo := b.GlobalCfg.MatchingRepo(criteria.OptionalPull.BaseRepo.ID())
	if !repo.BranchMatches(criteria.OptionalPull.BaseBranch) {
		return errors.New("command was run on a pull request which doesn't match base branches")
	}
	return nil
}
