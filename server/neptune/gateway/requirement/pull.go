package requirement

import (
	"context"

	"github.com/runatlantis/atlantis/server/models"
)

type pull struct{}

func (r pull) Check(ctx context.Context, criteria Criteria) error {
	if criteria.OptionalPull == nil {
		return nil
	}

	if criteria.OptionalPull.BaseRepo.Owner != criteria.OptionalPull.HeadRepo.Owner {
		return NewForbiddenError("pull request cannot be from a fork")
	}

	if criteria.OptionalPull.State == models.ClosedPullState {
		return NewForbiddenError("deploy cannot be executed on a closed pull request")
	}

	return nil
}
