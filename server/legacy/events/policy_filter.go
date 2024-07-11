package events

import (
	"context"

	gh "github.com/google/go-github/v45/github"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/legacy/events/command"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/models"
	"github.com/runatlantis/atlantis/server/neptune/lyft/feature"
)

type prReviewFetcher interface {
	ListLatestApprovalUsernames(ctx context.Context, installationToken int64, repo models.Repo, prNum int) ([]string, error)
	ListApprovalReviews(ctx context.Context, installationToken int64, repo models.Repo, prNum int) ([]*gh.PullRequestReview, error)
}

type prReviewDismisser interface {
	Dismiss(ctx context.Context, installationToken int64, repo models.Repo, prNum int, reviewID int64) error
}

type teamMemberFetcher interface {
	ListTeamMembers(ctx context.Context, installationToken int64, teamSlug string) ([]string, error)
}

type ApprovedPolicyFilter struct {
	prReviewDismisser prReviewDismisser
	prReviewFetcher   prReviewFetcher
	teamMemberFetcher teamMemberFetcher
	allocator         feature.Allocator
	policies          []valid.PolicySet
	logger            logging.Logger
}

func NewApprovedPolicyFilter(
	prReviewFetcher prReviewFetcher,
	prReviewDismisser prReviewDismisser,
	teamMemberFetcher teamMemberFetcher,
	allocator feature.Allocator,
	policySets []valid.PolicySet,
	logger logging.Logger) *ApprovedPolicyFilter {
	return &ApprovedPolicyFilter{
		prReviewFetcher:   prReviewFetcher,
		prReviewDismisser: prReviewDismisser,
		teamMemberFetcher: teamMemberFetcher,
		policies:          policySets,
		allocator:         allocator,
		logger:            logger,
	}
}

// Filter will remove failed policies if the underlying PR has been approved by a policy owner
func (p *ApprovedPolicyFilter) Filter(ctx context.Context, installationToken int64, repo models.Repo, prNum int, trigger command.CommandTrigger, failedPolicies []valid.PolicySet) ([]valid.PolicySet, error) {
	// Skip GH API calls if no policies failed
	if len(failedPolicies) == 0 {
		return failedPolicies, nil
	}

	// Fetch reviewers who approved the PR
	approvedReviewers, err := p.prReviewFetcher.ListLatestApprovalUsernames(ctx, installationToken, repo, prNum)
	if err != nil {
		return failedPolicies, errors.Wrap(err, "failed to fetch GH PR reviews")
	}

	// Filter out policies that already have been approved within GH
	var filteredFailedPolicies []valid.PolicySet
	for _, failedPolicy := range failedPolicies {
		approved, err := p.reviewersContainsPolicyOwner(ctx, installationToken, approvedReviewers, failedPolicy)
		if err != nil {
			return failedPolicies, errors.Wrap(err, "validating policy approval")
		}
		if !approved {
			filteredFailedPolicies = append(filteredFailedPolicies, failedPolicy)
		}
	}
	return filteredFailedPolicies, nil
}

func (p *ApprovedPolicyFilter) reviewersContainsPolicyOwner(ctx context.Context, installationToken int64, reviewers []string, policy valid.PolicySet) (bool, error) {
	// fetch owners from GH team
	owners, err := p.teamMemberFetcher.ListTeamMembers(ctx, installationToken, policy.Owner)
	if err != nil {
		return false, errors.Wrap(err, "failed to fetch GH team members")
	}

	// Check if any reviewer is an owner of the failed policy set
	for _, owner := range owners {
		for _, reviewer := range reviewers {
			if reviewer == owner {
				return true, nil
			}
		}
	}
	return false, nil
}
