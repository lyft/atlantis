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
type prReviewFetcher interface {
	ListReviews(ctx context.Context, installationToken int64, repo models.Repo, prNum int) ([]*gh.PullRequestReview, error)
}

type prReviewDismisser interface {
	Dismiss(ctx context.Context, installationToken int64, repo models.Repo, prNum int, reviewID int64) error
}

type teamMemberFetcher interface {
	ListTeamMembers(ctx context.Context, installationToken int64, teamSlug string) ([]string, error)
}

const ApprovalState = "APPROVED"

type ApprovedPolicyFilter struct {
	owners sync.Map //cache

	prReviewDismisser     prReviewDismisser
	prReviewFetcher       prReviewFetcher
	prLatestCommitFetcher prLatestCommitFetcher
	teamMemberFetcher     teamMemberFetcher
	policies              []valid.PolicySet
}

func NewApprovedPolicyFilter(
	prReviewFetcher prReviewFetcher,
	prReviewDismisser prReviewDismisser,
	prLatestCommitFetcher prLatestCommitFetcher,
	teamMemberFetcher teamMemberFetcher,
	policySets []valid.PolicySet) *ApprovedPolicyFilter {
	return &ApprovedPolicyFilter{
		prReviewFetcher:       prReviewFetcher,
		prReviewDismisser:     prReviewDismisser,
		prLatestCommitFetcher: prLatestCommitFetcher,
		teamMemberFetcher:     teamMemberFetcher,
		policies:              policySets,
		owners:                sync.Map{},
	}
}

// Filter will remove failed policies if the underlying PR has been approved by a policy owner
func (p *ApprovedPolicyFilter) Filter(ctx context.Context, installationToken int64, repo models.Repo, prNum int, failedPolicies []valid.PolicySet) ([]valid.PolicySet, error) {
	// Skip GH API calls if no policies failed
	if len(failedPolicies) == 0 {
		return failedPolicies, nil
	}

	// Fetch reviews from PR
	reviews, err := p.prReviewFetcher.ListReviews(ctx, installationToken, repo, prNum)
	if err != nil {
		return failedPolicies, errors.Wrap(err, "failed to fetch GH PR reviews")
	}

	// Need to dismiss stale reviews before determining which failed policies can be bypassed
	validReviews, err := p.dismissStalePRReviews(ctx, installationToken, repo, prNum, reviews)
	if err != nil {
		return failedPolicies, errors.Wrap(err, "failed to dismiss stale PR reviews")
	}

	latestApprovers := findLatestApprovalUsernames(validReviews)
	// Skip more potential GH calls if there are no valid approvers
	if len(latestApprovers) == 0 {
		return failedPolicies, nil
	}

	// Filter out policies that already have been approved within GH
	var filteredFailedPolicies []valid.PolicySet
	for _, failedPolicy := range failedPolicies {
		approved, err := p.reviewersContainsPolicyOwner(ctx, installationToken, latestApprovers, failedPolicy)
		if err != nil {
			return failedPolicies, errors.Wrap(err, "validating policy approval")
		}
		if !approved {
			filteredFailedPolicies = append(filteredFailedPolicies, failedPolicy)
		}
	}
	return filteredFailedPolicies, nil
}

func (p *ApprovedPolicyFilter) dismissStalePRReviews(ctx context.Context, installationToken int64, repo models.Repo, prNum int, reviews []*gh.PullRequestReview) ([]*gh.PullRequestReview, error) {
	latestCommitTimestamp, err := p.prLatestCommitFetcher.FetchLatestCommitTime(ctx, installationToken, repo, prNum)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch GH PR latest commit timestamp")
	}
	var validReviews []*gh.PullRequestReview
	for _, review := range reviews {
		// don't dismiss reviews that aren't approvals
		if review.GetState() != ApprovalState {
			validReviews = append(validReviews, review)
			continue
		}
		// don't dismiss approvals after latest commit
		if review.GetSubmittedAt().After(latestCommitTimestamp) {
			validReviews = append(validReviews, review)
			continue
		}
		isOwner, err := p.approverIsOwner(ctx, installationToken, review)
		if err != nil {
			return nil, errors.Wrap(err, "failed to validate approver is owner")
		}
		// don't dismiss approval unless from a policy owner
		if !isOwner {
			validReviews = append(validReviews, review)
			continue
		}
		err = p.prReviewDismisser.Dismiss(ctx, installationToken, repo, prNum, review.GetID())
		if err != nil {
			return nil, errors.Wrap(err, "failed to dismiss GH PR reviews")
		}
	}
	return validReviews, nil
}

func (p *ApprovedPolicyFilter) approverIsOwner(ctx context.Context, installationToken int64, approval *gh.PullRequestReview) (bool, error) {
	if approval.GetUser() == nil {
		return false, errors.New("failed to identify approver")
	}
	reviewers := []string{approval.GetUser().GetLogin()}
	for _, policy := range p.policies {
		isOwner, err := p.reviewersContainsPolicyOwner(ctx, installationToken, reviewers, policy)
		if err != nil {
			return false, errors.Wrap(err, "validating policy approval")
		}
		if isOwner {
			return true, nil
		}
	}
	return false, nil
}

func (p *ApprovedPolicyFilter) reviewersContainsPolicyOwner(ctx context.Context, installationToken int64, reviewers []string, policy valid.PolicySet) (bool, error) {
	// Check if any reviewer is an owner of the failed policy set
	if p.ownersContainsPolicy(reviewers, policy) {
		return true, nil
	}
	// Cache miss, fetch and try again
	err := p.fetchOwners(ctx, installationToken, policy)
	if err != nil {
		return false, errors.Wrap(err, "updating owners cache")
	}
	return p.ownersContainsPolicy(reviewers, policy), nil
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

func (p *ApprovedPolicyFilter) fetchOwners(ctx context.Context, installationToken int64, policy valid.PolicySet) error {
	members, err := p.teamMemberFetcher.ListTeamMembers(ctx, installationToken, policy.Owner)
	if err != nil {
		return errors.Wrap(err, "failed to fetch GH team members")
	}
	p.owners.Store(policy.Owner, members)
	return nil
}

// only return an approval from a user if it is their most recent review
// this is because a user can approve a PR then request more changes later on
func findLatestApprovalUsernames(reviews []*gh.PullRequestReview) []string {
	var approvalReviewers []string
	reviewers := make(map[string]bool)

	//reviews are returned chronologically
	for i := len(reviews) - 1; i >= 0; i-- {
		review := reviews[i]
		reviewer := review.GetUser()
		if reviewer == nil {
			continue
		}
		// add reviewer if an approval + we have not already processed their most recent review
		if review.GetState() == ApprovalState && !reviewers[reviewer.GetLogin()] {
			approvalReviewers = append(approvalReviewers, reviewer.GetLogin())
		}
		reviewers[reviewer.GetLogin()] = true
	}
	return approvalReviewers
}
