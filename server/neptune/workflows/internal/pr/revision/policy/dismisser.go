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

const dismissReason = "**New plans have triggered policy failures that must be approved by policy owners.**"

type StaleReviewDismisser struct {
	GithubActivities dismisserGithubActivities
	PRNumber         int
}

func (d *StaleReviewDismisser) Dismiss(ctx workflow.Context, revision revision.Revision, currentApprovals []*github.PullRequestReview) ([]*github.PullRequestReview, error) {
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
	var validApprovals []*github.PullRequestReview
	staleCommits := findStaleCommits(listCommitsResponse.Commits, revision.Revision)
	for _, approval := range currentApprovals {
		isValid := true
		for _, staleCommit := range staleCommits {
			if approval.GetCommitID() == staleCommit.GetSHA() {
				staleApprovals = append(staleApprovals, approval.GetID())
				isValid = false
				break
			}
		}
		if isValid {
			validApprovals = append(validApprovals, approval)
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
	return validApprovals, nil
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
