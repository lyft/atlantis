package policy

import (
	"context"
	"github.com/google/go-github/v45/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision"
	"go.temporal.io/sdk/workflow"
	"sort"
)

type dismisserGithubActivities interface {
	GithubListPRCommits(ctx context.Context, request activities.ListPRCommitsRequest) (activities.ListPRCommitsResponse, error)
	GithubDismiss(ctx context.Context, request activities.DismissRequest) (activities.DismissResponse, error)
}

const (
	ApprovalState = "APPROVED"
	dismissReason = "**New plans have triggered policy failures that must be approved by policy owners.**"
)

type StaleReviewDismisser struct {
	GithubActivities dismisserGithubActivities
	PRNumber         int
}

func (d *StaleReviewDismisser) Dismiss(ctx workflow.Context, revision revision.Revision, teams map[string][]string, currentReviews []*github.PullRequestReview) ([]*github.PullRequestReview, error) {
	// Fetch current commits on PR
	var listCommitsResponse activities.ListPRCommitsResponse
	err := workflow.ExecuteActivity(ctx, d.GithubActivities.GithubListPRCommits, activities.ListPRCommitsRequest{
		Repo:     revision.Repo,
		PRNumber: d.PRNumber,
	}).Get(ctx, &listCommitsResponse)
	if err != nil {
		return nil, err
	}

	// Filter for approvals on stale commits
	var staleApprovals []int64
	var validReviews []*github.PullRequestReview
	staleCommits := findStaleCommits(listCommitsResponse.Commits, revision.Revision)
	for _, review := range currentReviews {
		// only dismiss approvals
		if review.GetState() != ApprovalState {
			validReviews = append(validReviews, review)
			continue
		}
		// only dismiss reviews from policy owners
		if !approverIsOwner(review.GetUser().GetLogin(), teams) {
			validReviews = append(validReviews, review)
			continue
		}

		isValid := true
		for _, staleCommit := range staleCommits {
			if review.GetCommitID() == staleCommit.GetSHA() {
				staleApprovals = append(staleApprovals, review.GetID())
				isValid = false
				break
			}
		}
		if isValid {
			validReviews = append(validReviews, review)
		}
	}

	// Dismiss each stale approval
	for _, staleApproval := range staleApprovals {
		err = workflow.ExecuteActivity(ctx, d.GithubActivities.GithubDismiss, activities.DismissRequest{
			Repo:          revision.Repo,
			PRNumber:      d.PRNumber,
			ReviewID:      staleApproval,
			DismissReason: dismissReason,
		}).Get(ctx, nil)
		if err != nil {
			return nil, err
		}
	}
	return validReviews, nil
}

func approverIsOwner(approver string, teams map[string][]string) bool {
	for _, team := range teams {
		for _, member := range team {
			if member == approver {
				return true
			}
		}
	}
	return false
}

// sorts commits by timestamp and returns all commits that are older than the current revision
func findStaleCommits(commits []*github.RepositoryCommit, currentRevision string) []*github.RepositoryCommit {
	sort.Sort(ByTimestamp(commits))
	var staleCommits []*github.RepositoryCommit
	for _, commit := range commits {
		if commit.GetSHA() == currentRevision {
			break
		}
		staleCommits = append(staleCommits, commit)
	}
	return staleCommits
}

type ByTimestamp []*github.RepositoryCommit

func (c ByTimestamp) Len() int {
	return len(c)
}

func (c ByTimestamp) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

func (c ByTimestamp) Less(i, j int) bool {
	timeI := c[i].GetCommit().GetCommitter().GetDate()
	timeJ := c[j].GetCommit().GetCommitter().GetDate()
	return timeI.Before(timeJ)
}
