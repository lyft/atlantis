package requirement

import (
	"context"

	"github.com/google/go-github/v45/github"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/neptune/template"
)

type reviewFetcher interface {
	ListApprovalReviews(ctx context.Context, installationToken int64, repo models.Repo, prNum int) ([]*github.PullRequestReview, error)
}

type approval struct {
	cfg            valid.GlobalCfg
	fetcher        reviewFetcher
	errorGenerator errGenerator[template.Input]
}

func (a approval) Check(ctx context.Context, criteria Criteria) error {
	match := a.cfg.MatchingRepo(criteria.Repo.ID())

	shouldCheckApproval := match.ApplySettings.ContainsPRRequirement(valid.ApprovedApplyReq)

	if !shouldCheckApproval || criteria.OptionalPull == nil {
		return nil
	}

	reviews, err := a.fetcher.ListApprovalReviews(ctx, criteria.InstallationToken, criteria.Repo, criteria.OptionalPull.Num)

	if err != nil {
		return errors.Wrap(err, "fetching approval reviews")
	}

	if len(reviews) > 0 {
		return nil
	}

	return a.errorGenerator.GenerateForbiddenError(ctx, template.ApprovalRequired, criteria.Repo, template.Input{},
		"PR approval is required",
	)
}
