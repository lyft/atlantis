package policy_test

import (
	"github.com/google/go-github/v45/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision/policy"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFilter_FilterOutTeamA(t *testing.T) {
	filter := policy.Filter{}
	expectedFilteredPolicies := []activities.PolicySet{
		{
			Owner: "team-c",
		},
	}
	teams := map[string][]string{
		"team-a": {"owner-a1", "owner-a2"},
		"team-b": {"owner-b1", "owner-b2"},
		"team-c": {"owner-c1", "owner-c2"},
	}
	failedPolicies := []activities.PolicySet{
		{
			Owner: "team-a",
		},
		{
			Owner: "team-c",
		},
	}
	currentReviews := []*github.PullRequestReview{
		{
			User: &github.User{
				Login: github.String("owner-a1"),
			},
			State: github.String(policy.ApprovalState),
		},
		{
			User: &github.User{
				Login: github.String("another-user"),
			},
			State: github.String(policy.ApprovalState),
		},
	}
	filteredPolicies := filter.Filter(teams, currentReviews, failedPolicies)
	assert.Equal(t, expectedFilteredPolicies, filteredPolicies)
}

func TestFilter_IgnoreOldApprovalWhenChangesRequested(t *testing.T) {
	filter := policy.Filter{}
	teams := map[string][]string{
		"team-a": {"owner-a1", "owner-a2"},
		"team-b": {"owner-b1", "owner-b2"},
		"team-c": {"owner-c1", "owner-c2"},
	}
	failedPolicies := []activities.PolicySet{
		{
			Owner: "team-a",
		},
		{
			Owner: "team-c",
		},
	}
	currentReviews := []*github.PullRequestReview{
		{
			User: &github.User{
				Login: github.String("owner-a1"),
			},
			State: github.String(policy.ApprovalState),
		},
		{
			User: &github.User{
				Login: github.String("owner-a1"),
			},
			State: github.String("CHANGES_REQUESTED"),
		},
	}
	filteredPolicies := filter.Filter(teams, currentReviews, failedPolicies)
	assert.Equal(t, failedPolicies, filteredPolicies)
}
