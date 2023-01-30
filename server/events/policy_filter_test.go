package events

// not using a separate test package to be able to test some private fields in struct ApprovedPolicyFilter

import (
	"context"
	"github.com/google/go-github/v45/github"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

const (
	ownerA      = "A"
	ownerB      = "B"
	ownerC      = "C"
	policyName  = "some-policy"
	policyOwner = "team"
)

func TestFilter_Approved(t *testing.T) {
	time1 := time.UnixMicro(1)
	time2 := time.UnixMicro(2)

	reviewFetcher := &mockReviewFetcher{
		reviews: []*github.PullRequestReview{
			{
				User:  &github.User{Login: github.String(ownerA)},
				State: github.String(ApprovalState),
			},
			{
				User:        &github.User{Login: github.String(ownerB)},
				SubmittedAt: &time2,
				State:       github.String(ApprovalState),
			},
		},
	}
	teamFetcher := &mockTeamMemberFetcher{
		members: []string{ownerA, ownerB, ownerC},
	}
	commitFetcher := &mockCommitFetcher{
		time: time1,
	}
	reviewDismisser := &mockReviewDismisser{}
	failedPolicies := []valid.PolicySet{
		{Name: policyName, Owner: policyOwner},
	}

	policyFilter := NewApprovedPolicyFilter(reviewFetcher, reviewDismisser, commitFetcher, teamFetcher, failedPolicies)
	filteredPolicies, err := policyFilter.Filter(context.Background(), 0, models.Repo{}, 0, failedPolicies)
	assert.NoError(t, err)
	assert.True(t, reviewFetcher.isCalled)
	assert.True(t, commitFetcher.isCalled)
	assert.True(t, teamFetcher.isCalled)
	assert.True(t, reviewDismisser.isCalled)
	assert.Empty(t, filteredPolicies)
}

func TestFilter_Approved_NoDismissal(t *testing.T) {
	time1 := time.UnixMicro(1)
	time2 := time.UnixMicro(2)
	reviewFetcher := &mockReviewFetcher{
		reviews: []*github.PullRequestReview{
			{
				User:        &github.User{Login: github.String(ownerB)},
				SubmittedAt: &time2,
				State:       github.String(ApprovalState),
			},
		},
	}
	commitFetcher := &mockCommitFetcher{
		time: time1,
	}
	reviewDismisser := &mockReviewDismisser{}
	teamFetcher := &mockTeamMemberFetcher{
		members: []string{ownerA, ownerB, ownerC},
	}
	failedPolicies := []valid.PolicySet{
		{Name: policyName, Owner: policyOwner},
	}

	policyFilter := NewApprovedPolicyFilter(reviewFetcher, reviewDismisser, commitFetcher, teamFetcher, failedPolicies)
	filteredPolicies, err := policyFilter.Filter(context.Background(), 0, models.Repo{}, 0, failedPolicies)
	assert.NoError(t, err)
	assert.True(t, reviewFetcher.isCalled)
	assert.True(t, commitFetcher.isCalled)
	assert.True(t, teamFetcher.isCalled)
	assert.False(t, reviewDismisser.isCalled)
	assert.Empty(t, filteredPolicies)
}

func TestFilter_Approved_PreFilledCache(t *testing.T) {
	time1 := time.UnixMicro(1)
	team := []string{ownerA, ownerB, ownerC}
	reviewFetcher := &mockReviewFetcher{
		reviews: []*github.PullRequestReview{
			{
				User:        &github.User{Login: github.String(ownerB)},
				State:       github.String(ApprovalState),
				SubmittedAt: &time1,
			},
		},
	}
	teamFetcher := &mockTeamMemberFetcher{
		members: team,
	}
	commitFetcher := &mockCommitFetcher{}
	reviewDismisser := &mockReviewDismisser{}
	failedPolicies := []valid.PolicySet{
		{Name: policyName, Owner: policyOwner},
	}

	policyFilter := NewApprovedPolicyFilter(reviewFetcher, reviewDismisser, commitFetcher, teamFetcher, failedPolicies)
	policyFilter.owners.Store(policyOwner, team)
	filteredPolicies, err := policyFilter.Filter(context.Background(), 0, models.Repo{}, 0, failedPolicies)
	assert.NoError(t, err)
	assert.True(t, reviewFetcher.isCalled)
	assert.True(t, commitFetcher.isCalled)
	assert.False(t, reviewDismisser.isCalled)
	assert.False(t, teamFetcher.isCalled)
	assert.Empty(t, filteredPolicies)
}

func TestFilter_NotApproved_Dismissal(t *testing.T) {
	time1 := time.UnixMicro(1)
	time2 := time.UnixMicro(2)

	reviewFetcher := &mockReviewFetcher{
		reviews: []*github.PullRequestReview{
			{
				User:        &github.User{Login: github.String(ownerA)},
				State:       github.String(ApprovalState),
				SubmittedAt: &time1,
			},
		},
	}
	teamFetcher := &mockTeamMemberFetcher{
		members: []string{ownerA, ownerB, ownerC},
	}
	commitFetcher := &mockCommitFetcher{
		time: time2,
	}
	reviewDismisser := &mockReviewDismisser{}
	failedPolicies := []valid.PolicySet{
		{Name: policyName, Owner: policyOwner},
	}

	policyFilter := NewApprovedPolicyFilter(reviewFetcher, reviewDismisser, commitFetcher, teamFetcher, failedPolicies)
	filteredPolicies, err := policyFilter.Filter(context.Background(), 0, models.Repo{}, 0, failedPolicies)
	assert.NoError(t, err)
	assert.True(t, reviewFetcher.isCalled)
	assert.True(t, commitFetcher.isCalled)
	assert.True(t, reviewDismisser.isCalled)
	assert.True(t, teamFetcher.isCalled)
	assert.Equal(t, failedPolicies, filteredPolicies)
}

func TestFilter_NotApproved_NotOwner(t *testing.T) {
	reviewFetcher := &mockReviewFetcher{
		reviews: []*github.PullRequestReview{
			{
				User:  &github.User{Login: github.String(ownerB)},
				State: github.String(ApprovalState),
			},
		},
	}
	teamFetcher := &mockTeamMemberFetcher{
		members: []string{ownerA, ownerC},
	}
	commitFetcher := &mockCommitFetcher{}
	reviewDismisser := &mockReviewDismisser{}
	failedPolicies := []valid.PolicySet{
		{Name: policyName, Owner: policyOwner},
	}

	policyFilter := NewApprovedPolicyFilter(reviewFetcher, reviewDismisser, commitFetcher, teamFetcher, failedPolicies)
	filteredPolicies, err := policyFilter.Filter(context.Background(), 0, models.Repo{}, 0, failedPolicies)
	assert.NoError(t, err)
	assert.True(t, reviewFetcher.isCalled)
	assert.True(t, commitFetcher.isCalled)
	assert.False(t, reviewDismisser.isCalled)
	assert.True(t, teamFetcher.isCalled)
	assert.Equal(t, failedPolicies, filteredPolicies)
}

func TestFilter_NotApproved_FutureChangesRequest(t *testing.T) {
	time1 := time.UnixMicro(1)
	time2 := time.UnixMicro(2)

	reviewFetcher := &mockReviewFetcher{
		reviews: []*github.PullRequestReview{
			{
				User:        &github.User{Login: github.String(ownerA)},
				State:       github.String(ApprovalState),
				SubmittedAt: &time1,
			},
			{
				User:        &github.User{Login: github.String(ownerA)},
				State:       github.String("Request Changes"),
				SubmittedAt: &time2,
			},
		},
	}
	teamFetcher := &mockTeamMemberFetcher{
		members: []string{ownerA, ownerB, ownerC},
	}
	commitFetcher := &mockCommitFetcher{}
	reviewDismisser := &mockReviewDismisser{}
	failedPolicies := []valid.PolicySet{
		{Name: policyName, Owner: policyOwner},
	}

	policyFilter := NewApprovedPolicyFilter(reviewFetcher, reviewDismisser, commitFetcher, teamFetcher, failedPolicies)
	filteredPolicies, err := policyFilter.Filter(context.Background(), 0, models.Repo{}, 0, failedPolicies)
	assert.NoError(t, err)
	assert.True(t, reviewFetcher.isCalled)
	assert.True(t, commitFetcher.isCalled)
	assert.False(t, reviewDismisser.isCalled)
	assert.False(t, teamFetcher.isCalled)
	assert.Equal(t, failedPolicies, filteredPolicies)
}

func TestFilter_NoFailedPolicies(t *testing.T) {
	reviewFetcher := &mockReviewFetcher{}
	teamFetcher := &mockTeamMemberFetcher{}
	commitFetcher := &mockCommitFetcher{}
	reviewDismisser := &mockReviewDismisser{}

	var failedPolicies []valid.PolicySet
	policyFilter := NewApprovedPolicyFilter(reviewFetcher, reviewDismisser, commitFetcher, teamFetcher, failedPolicies)
	filteredPolicies, err := policyFilter.Filter(context.Background(), 0, models.Repo{}, 0, failedPolicies)
	assert.NoError(t, err)
	assert.False(t, reviewFetcher.isCalled)
	assert.Empty(t, filteredPolicies)
}

func TestFilter_FailedListReviews(t *testing.T) {
	reviewFetcher := &mockReviewFetcher{
		error: assert.AnError,
	}
	teamFetcher := &mockTeamMemberFetcher{
		members: []string{ownerA, ownerB, ownerC},
	}
	commitFetcher := &mockCommitFetcher{}
	reviewDismisser := &mockReviewDismisser{}
	failedPolicies := []valid.PolicySet{
		{Name: policyName, Owner: policyOwner},
	}

	policyFilter := NewApprovedPolicyFilter(reviewFetcher, reviewDismisser, commitFetcher, teamFetcher, failedPolicies)
	filteredPolicies, err := policyFilter.Filter(context.Background(), 0, models.Repo{}, 0, failedPolicies)
	assert.ErrorIs(t, err, assert.AnError)
	assert.True(t, reviewFetcher.isCalled)
	assert.False(t, commitFetcher.isCalled)
	assert.Equal(t, failedPolicies, filteredPolicies)
}

func TestFilter_FailedFetchLatestCommitTime(t *testing.T) {
	reviewFetcher := &mockReviewFetcher{
		reviews: []*github.PullRequestReview{
			{
				User:  &github.User{Login: github.String(ownerB)},
				State: github.String(ApprovalState),
			},
		},
	}
	teamFetcher := &mockTeamMemberFetcher{}
	commitFetcher := &mockCommitFetcher{
		error: assert.AnError,
	}
	reviewDismisser := &mockReviewDismisser{}

	failedPolicies := []valid.PolicySet{
		{Name: policyName, Owner: policyOwner},
	}
	policyFilter := NewApprovedPolicyFilter(reviewFetcher, reviewDismisser, commitFetcher, teamFetcher, failedPolicies)
	filteredPolicies, err := policyFilter.Filter(context.Background(), 0, models.Repo{}, 0, failedPolicies)
	assert.ErrorIs(t, err, assert.AnError)
	assert.True(t, reviewFetcher.isCalled)
	assert.True(t, commitFetcher.isCalled)
	assert.False(t, teamFetcher.isCalled)
	assert.False(t, reviewDismisser.isCalled)
	assert.Equal(t, failedPolicies, filteredPolicies)
}

func TestFilter_FailedTeamMemberFetch(t *testing.T) {
	reviewFetcher := &mockReviewFetcher{
		reviews: []*github.PullRequestReview{
			{
				User:  &github.User{Login: github.String(ownerB)},
				State: github.String(ApprovalState),
			},
		},
	}
	teamFetcher := &mockTeamMemberFetcher{
		error: assert.AnError,
	}
	commitFetcher := &mockCommitFetcher{}
	reviewDismisser := &mockReviewDismisser{}
	failedPolicies := []valid.PolicySet{
		{Name: policyName, Owner: policyOwner},
	}

	policyFilter := NewApprovedPolicyFilter(reviewFetcher, reviewDismisser, commitFetcher, teamFetcher, failedPolicies)
	filteredPolicies, err := policyFilter.Filter(context.Background(), 0, models.Repo{}, 0, failedPolicies)
	assert.ErrorIs(t, err, assert.AnError)
	assert.True(t, reviewFetcher.isCalled)
	assert.True(t, commitFetcher.isCalled)
	assert.True(t, teamFetcher.isCalled)
	assert.False(t, reviewDismisser.isCalled)
	assert.Equal(t, failedPolicies, filteredPolicies)
}

func TestFilter_FailedDismiss(t *testing.T) {
	time1 := time.UnixMicro(1)
	time2 := time.UnixMicro(2)
	reviewFetcher := &mockReviewFetcher{
		reviews: []*github.PullRequestReview{
			{
				User:        &github.User{Login: github.String(ownerB)},
				SubmittedAt: &time1,
				State:       github.String(ApprovalState),
			},
		},
	}
	commitFetcher := &mockCommitFetcher{
		time: time2,
	}
	reviewDismisser := &mockReviewDismisser{
		error: assert.AnError,
	}
	teamFetcher := &mockTeamMemberFetcher{
		members: []string{ownerA, ownerB, ownerC},
	}
	failedPolicies := []valid.PolicySet{
		{Name: policyName, Owner: policyOwner},
	}

	policyFilter := NewApprovedPolicyFilter(reviewFetcher, reviewDismisser, commitFetcher, teamFetcher, failedPolicies)
	filteredPolicies, err := policyFilter.Filter(context.Background(), 0, models.Repo{}, 0, failedPolicies)
	assert.ErrorIs(t, err, assert.AnError)
	assert.True(t, reviewFetcher.isCalled)
	assert.True(t, commitFetcher.isCalled)
	assert.True(t, teamFetcher.isCalled)
	assert.True(t, reviewDismisser.isCalled)
	assert.Equal(t, failedPolicies, filteredPolicies)
}

type mockReviewFetcher struct {
	reviews  []*github.PullRequestReview
	isCalled bool
	error    error
}

func (f *mockReviewFetcher) ListReviews(_ context.Context, _ int64, _ models.Repo, _ int) ([]*github.PullRequestReview, error) {
	f.isCalled = true
	return f.reviews, f.error
}

type mockCommitFetcher struct {
	time     time.Time
	error    error
	isCalled bool
}

func (c *mockCommitFetcher) FetchLatestCommitTime(_ context.Context, _ int64, _ models.Repo, _ int) (time.Time, error) {
	c.isCalled = true
	return c.time, c.error
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
