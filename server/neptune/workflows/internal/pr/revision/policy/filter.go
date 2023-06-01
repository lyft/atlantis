package policy

import (
	"github.com/google/go-github/v45/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
)

type Filter struct{}

func (p *Filter) Filter(teams map[string][]string, currentApprovals []*github.PullRequestReview, failedPolicies []activities.PolicySet) []activities.PolicySet {
	approvedReviewers := findLatestApprovalUsernames(currentApprovals)

	var filteredFailedPolicies []activities.PolicySet
	for _, failedPolicy := range failedPolicies {
		if approved := p.reviewersContainsPolicyOwner(approvedReviewers, teams, failedPolicy); !approved {
			filteredFailedPolicies = append(filteredFailedPolicies, failedPolicy)
		}
	}
	return filteredFailedPolicies
}

func (p *Filter) reviewersContainsPolicyOwner(reviewers []string, teams map[string][]string, policy activities.PolicySet) bool {
	// Check if any reviewer is an owner of the failed policy set
	owners := teams[policy.Owner]
	for _, owner := range owners {
		for _, reviewer := range reviewers {
			if reviewer == owner {
				return true
			}
		}
	}
	return false
}

// only return an approval from a user if it is their most recent review
// this is because a user can approve a PR then request more changes later on
func findLatestApprovalUsernames(reviews []*github.PullRequestReview) []string {
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
		if !reviewers[reviewer.GetLogin()] {
			approvalReviewers = append(approvalReviewers, reviewer.GetLogin())
		}
		reviewers[reviewer.GetLogin()] = true
	}
	return approvalReviewers
}
