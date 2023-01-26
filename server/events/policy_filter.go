package events

import (
	"context"
	gh "github.com/google/go-github/v45/github"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models"
	"sync"
	"time"
)

type prLatestCommitFetcher interface {
	FetchLatestCommitTime(ctx context.Context, installationToken int64, repo models.Repo, prNum int) (time.Time, error)
}
type prReviewsFetcher interface {
	// TODO: explore merging the two/performing on GH api call
	ListApprovalReviewers(ctx context.Context, installationToken int64, repo models.Repo, prNum int) ([]string, error)
	ListApprovalReviews(ctx context.Context, installationToken int64, repo models.Repo, prNum int) ([]*gh.PullRequestReview, error)
}

type prReviewDismisser interface {
	Dismiss(ctx context.Context, installationToken int64, repo models.Repo, prNum int, reviewID int64) error
}

type teamMemberFetcher interface {
	ListTeamMembers(ctx context.Context, installationToken int64, teamSlug string) ([]string, error)
}

type ApprovedPolicyFilter struct {
	owners sync.Map //cache

	prReviewDismisser     prReviewDismisser
	prReviewsFetcher      prReviewsFetcher
	prLatestCommitFetcher prLatestCommitFetcher
	teamMemberFetcher     teamMemberFetcher
}

func NewApprovedPolicyFilter(
	prReviewsFetcher prReviewsFetcher,
	prReviewDismisser prReviewDismisser,
	prLatestCommitFetcher prLatestCommitFetcher,
	teamMemberFetcher teamMemberFetcher) *ApprovedPolicyFilter {
	return &ApprovedPolicyFilter{
		prReviewsFetcher:      prReviewsFetcher,
		prReviewDismisser:     prReviewDismisser,
		prLatestCommitFetcher: prLatestCommitFetcher,
		teamMemberFetcher:     teamMemberFetcher,
		owners:                sync.Map{},
	}
}

// Filter will remove failed policies if the underlying PR has been approved by a policy owner
func (p *ApprovedPolicyFilter) Filter(ctx context.Context, installationToken int64, repo models.Repo, prNum int, failedPolicies []valid.PolicySet) ([]valid.PolicySet, error) {
	// Skip GH API calls if no policies failed
	if len(failedPolicies) == 0 {
		return failedPolicies, nil
	}

	// Need to dismiss stale reviews before determining which failed policies can be bypassed
	err := p.dismissStalePRReviews(ctx, installationToken, repo, prNum)
	if err != nil {
		return failedPolicies, errors.Wrap(err, "failed to dismiss stale PR reviews")
	}

	// Fetch reviewers who approved the PR
	approvedReviewers, err := p.prReviewsFetcher.ListApprovalReviewers(ctx, installationToken, repo, prNum)
	if err != nil {
		return failedPolicies, errors.Wrap(err, "failed to fetch GH PR reviews")
	}

	// Filter out policies that already have been approved within GH
	var filteredFailedPolicies []valid.PolicySet
	for _, failedPolicy := range failedPolicies {
		approved, err := p.policyApproved(ctx, installationToken, approvedReviewers, failedPolicy)
		if err != nil {
			return failedPolicies, errors.Wrap(err, "validating policy approval")
		}
		if !approved {
			filteredFailedPolicies = append(filteredFailedPolicies, failedPolicy)
		}
	}
	return filteredFailedPolicies, nil
}

func (p *ApprovedPolicyFilter) policyApproved(ctx context.Context, installationToken int64, approvedReviewers []string, failedPolicy valid.PolicySet) (bool, error) {
	// Check if any reviewer is an owner of the failed policy set
	if p.ownersContainsPolicy(approvedReviewers, failedPolicy) {
		return true, nil
	}

	// Cache miss, fetch and try again
	err := p.getOwners(ctx, installationToken, failedPolicy)
	if err != nil {
		return false, errors.Wrap(err, "updating owners cache")
	}
	return p.ownersContainsPolicy(approvedReviewers, failedPolicy), nil
}

func (p *ApprovedPolicyFilter) ownersContainsPolicy(approvedReviewers []string, failedPolicy valid.PolicySet) bool {
	// Check if any reviewer is an owner of the failed policy set
	owners, ok := p.owners.Load(failedPolicy.Owner)
	if !ok {
		return false
	}
	for _, owner := range owners.([]string) {
		for _, reviewer := range approvedReviewers {
			if reviewer == owner {
				return true
			}
		}
	}
	return false
}

func (p *ApprovedPolicyFilter) getOwners(ctx context.Context, installationToken int64, policy valid.PolicySet) error {
	members, err := p.teamMemberFetcher.ListTeamMembers(ctx, installationToken, policy.Owner)
	if err != nil {
		return errors.Wrap(err, "failed to fetch GH team members")
	}
	p.owners.Store(policy.Owner, members)
	return nil
}

func (p *ApprovedPolicyFilter) dismissStalePRReviews(ctx context.Context, installationToken int64, repo models.Repo, prNum int) error {
	approvalReviews, err := p.prReviewsFetcher.ListApprovalReviews(ctx, installationToken, repo, prNum)
	if err != nil {
		return errors.Wrap(err, "failed to fetch GH PR reviews")
	}

	latestCommitTimestamp, err := p.prLatestCommitFetcher.FetchLatestCommitTime(ctx, installationToken, repo, prNum)
	if err != nil {
		return errors.Wrap(err, "failed to fetch GH PR latest commit timestamp")
	}

	for _, approval := range approvalReviews {
		if approval.GetUser() == nil {
			return errors.New("failed to identify approver")
		}
		if p.approverIsOwner(approval.GetUser().GetLogin()) && approval.GetSubmittedAt().Before(latestCommitTimestamp) {
			err := p.prReviewDismisser.Dismiss(ctx, installationToken, repo, prNum, approval.GetID())
			if err != nil {
				return errors.Wrap(err, "failed to dismiss GH PR reviews")
			}
		}
	}
	return nil
}

// TODO: don't rely on a potentially stale cache?
func (p *ApprovedPolicyFilter) approverIsOwner(approver string) bool {
	isOwner := false
	p.owners.Range(func(key, value any) bool {
		teamMembers := value.([]string)
		for _, member := range teamMembers {
			if member == approver {
				isOwner = true
				return false
			}
		}
		return true
	})
	return isOwner
}
