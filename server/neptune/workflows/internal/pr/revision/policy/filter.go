package policy

import (
	"github.com/google/go-github/v45/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
)

type Filter struct{}

func (p *Filter) Filter(teams map[string][]string, currentReviews []*github.PullRequestReview, failedPolicies []activities.PolicySet) []activities.PolicySet {
	approvers := findRecentApprovers(currentReviews)
	var filteredFailedPolicies []activities.PolicySet
	for _, failedPolicy := range failedPolicies {
		if approved := p.policyApprovedByOwner(approvers, teams[failedPolicy.Owner]); !approved {
			filteredFailedPolicies = append(filteredFailedPolicies, failedPolicy)
		}
	}
	return filteredFailedPolicies
}

// only return an approval from a user if it is their most recent review
// this is because a user can approve a PR then request more changes later on
func findRecentApprovers(reviews []*github.PullRequestReview) []string {
	var approvalReviewers []string
	reviewers := make(map[string]bool)

	//reviews are returned chronologically
	for i := len(reviews) - 1; i >= 0; i-- {
		review := reviews[i]
		reviewer := review.GetUser()
		if reviewer == nil {
			continue
		}
		// add reviewer if: review = an approval + we have not already processed their most recent review
		if review.GetState() == ApprovalState && !reviewers[reviewer.GetLogin()] {
			approvalReviewers = append(approvalReviewers, reviewer.GetLogin())
		}
		reviewers[reviewer.GetLogin()] = true
	}
	return approvalReviewers
}

func (p *Filter) policyApprovedByOwner(approvers []string, owners []string) bool {
	// Check if any reviewer is an owner of the failed policy set
	for _, owner := range owners {
		for _, approver := range approvers {
			if approver == owner {
				return true
			}
		}
	}
	return false
}
