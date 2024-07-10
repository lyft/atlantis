package events

// not using a separate test package to be able to test some private fields in struct ApprovedPolicyFilter

import (
	"context"
	"testing"

	"github.com/google/go-github/v45/github"
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/legacy/events/command"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/models"
	"github.com/runatlantis/atlantis/server/neptune/lyft/feature"
	"github.com/stretchr/testify/assert"
)

const (
	ownerA      = "A"
	ownerB      = "B"
	ownerC      = "C"
	policyName  = "some-policy"
	policyOwner = "team"
)

func TestFilter_Approved(t *testing.T) {
	reviewFetcher := &mockReviewFetcher{
		approvers: []string{ownerB},
	}
	reviewDismisser := &mockReviewDismisser{}
	teamFetcher := &mockTeamMemberFetcher{
		members: []string{ownerA, ownerB, ownerC},
	}
	failedPolicies := []valid.PolicySet{
		{Name: policyName, Owner: policyOwner},
	}

	policyFilter := NewApprovedPolicyFilter(reviewFetcher, reviewDismisser, teamFetcher, &testFeatureAllocator{}, failedPolicies, logging.NewNoopCtxLogger(t))
	filteredPolicies, err := policyFilter.Filter(context.Background(), 0, models.Repo{}, 0, command.PRReviewTrigger, failedPolicies)
	assert.NoError(t, err)
	assert.True(t, reviewFetcher.listUsernamesIsCalled)
	assert.False(t, reviewFetcher.listApprovalsIsCalled)
	assert.True(t, teamFetcher.isCalled)
	assert.False(t, reviewDismisser.isCalled)
	assert.Empty(t, filteredPolicies)
}

func TestFilter_NoFailedPolicies(t *testing.T) {
	reviewFetcher := &mockReviewFetcher{
		approvers: []string{ownerB},
	}
	teamFetcher := &mockTeamMemberFetcher{
		members: []string{ownerA, ownerB, ownerC},
	}
	reviewDismisser := &mockReviewDismisser{}

	var failedPolicies []valid.PolicySet
	policyFilter := NewApprovedPolicyFilter(reviewFetcher, reviewDismisser, teamFetcher, &testFeatureAllocator{}, failedPolicies, logging.NewNoopCtxLogger(t))
	filteredPolicies, err := policyFilter.Filter(context.Background(), 0, models.Repo{}, 0, command.PRReviewTrigger, failedPolicies)
	assert.NoError(t, err)
	assert.False(t, reviewFetcher.listUsernamesIsCalled)
	assert.False(t, reviewFetcher.listApprovalsIsCalled)
	assert.False(t, teamFetcher.isCalled)
	assert.False(t, reviewDismisser.isCalled)
	assert.Empty(t, filteredPolicies)
}

func TestFilter_FailedListLatestApprovalUsernames(t *testing.T) {
	reviewFetcher := &mockReviewFetcher{
		listUsernamesError: assert.AnError,
	}
	teamFetcher := &mockTeamMemberFetcher{}
	reviewDismisser := &mockReviewDismisser{}
	failedPolicies := []valid.PolicySet{
		{Name: policyName, Owner: policyOwner},
	}

	policyFilter := NewApprovedPolicyFilter(reviewFetcher, reviewDismisser, teamFetcher, &testFeatureAllocator{}, failedPolicies, logging.NewNoopCtxLogger(t))
	filteredPolicies, err := policyFilter.Filter(context.Background(), 0, models.Repo{}, 0, command.PRReviewTrigger, failedPolicies)
	assert.Error(t, err)
	assert.True(t, reviewFetcher.listUsernamesIsCalled)
	assert.False(t, reviewFetcher.listApprovalsIsCalled)
	assert.False(t, reviewDismisser.isCalled)
	assert.False(t, teamFetcher.isCalled)
	assert.Equal(t, failedPolicies, filteredPolicies)
}

func TestFilter_FailedTeamMemberFetch(t *testing.T) {
	reviewFetcher := &mockReviewFetcher{
		approvers: []string{ownerB},
	}
	teamFetcher := &mockTeamMemberFetcher{
		error: assert.AnError,
	}
	reviewDismisser := &mockReviewDismisser{}
	failedPolicies := []valid.PolicySet{
		{Name: policyName, Owner: policyOwner},
	}

	policyFilter := NewApprovedPolicyFilter(reviewFetcher, reviewDismisser, teamFetcher, &testFeatureAllocator{}, failedPolicies, logging.NewNoopCtxLogger(t))
	filteredPolicies, err := policyFilter.Filter(context.Background(), 0, models.Repo{}, 0, command.PRReviewTrigger, failedPolicies)
	assert.Error(t, err)
	assert.True(t, reviewFetcher.listUsernamesIsCalled)
	assert.False(t, reviewFetcher.listApprovalsIsCalled)
	assert.True(t, teamFetcher.isCalled)
	assert.False(t, reviewDismisser.isCalled)
	assert.Equal(t, failedPolicies, filteredPolicies)
}

type mockReviewFetcher struct {
	approvers             []string
	listUsernamesIsCalled bool
	listUsernamesError    error
	reviews               []*github.PullRequestReview
	listApprovalsIsCalled bool
	listApprovalsError    error
}

func (f *mockReviewFetcher) ListLatestApprovalUsernames(_ context.Context, _ int64, _ models.Repo, _ int) ([]string, error) {
	f.listUsernamesIsCalled = true
	return f.approvers, f.listUsernamesError
}

func (f *mockReviewFetcher) ListApprovalReviews(_ context.Context, _ int64, _ models.Repo, _ int) ([]*github.PullRequestReview, error) {
	f.listApprovalsIsCalled = true
	return f.reviews, f.listApprovalsError
}

type mockReviewDismisser struct {
	error    error
	isCalled bool
}

func (d *mockReviewDismisser) Dismiss(_ context.Context, _ int64, _ models.Repo, _ int, _ int64) error {
	d.isCalled = true
	return d.error
}

type mockTeamMemberFetcher struct {
	members  []string
	error    error
	isCalled bool
}

func (m *mockTeamMemberFetcher) ListTeamMembers(_ context.Context, _ int64, _ string) ([]string, error) {
	m.isCalled = true
	return m.members, m.error
}

type testFeatureAllocator struct {
	Enabled bool
	Err     error
}

func (t *testFeatureAllocator) ShouldAllocate(featureID feature.Name, featureCtx feature.FeatureContext) (bool, error) {
	return t.Enabled, t.Err
}
