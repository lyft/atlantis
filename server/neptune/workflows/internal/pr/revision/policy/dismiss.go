package policy

import (
	"context"
	"github.com/google/go-github/v45/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision"
	"go.temporal.io/sdk/workflow"
	"sort"
)

const dismissReason = "**New plans have triggered policy failures that must be approved by policy owners.**"

type githubActivities interface {
	GithubListPRCommits(ctx context.Context, request activities.ListPRCommitsRequest) (activities.ListPRCommitsResponse, error)
	GithubListPRApprovals(ctx context.Context, request activities.ListPRApprovalsRequest) (activities.ListPRApprovalsResponse, error)
	GithubDismiss(ctx context.Context, request activities.DismissRequest) (activities.DismissResponse, error)
}

type StaleReviewDismisser struct {
	githubActivities githubActivities
	PRNumber         int
}

func (d *StaleReviewDismisser) Dismiss(ctx workflow.Context, revision revision.Revision) error {
	// Fetch current commits on PR
	var listCommitsResponse activities.ListPRCommitsResponse
	err := workflow.ExecuteActivity(ctx, d.githubActivities.GithubListPRCommits, activities.ListPRCommitsRequest{
		Repo:     revision.Repo,
		PRNumber: d.PRNumber,
	}).Get(ctx, &listCommitsResponse)
	if err != nil {
		return err
	}

	// Fetch current approvals in activity
	var listPRApprovalsResponse activities.ListPRApprovalsResponse
	err = workflow.ExecuteActivity(ctx, d.githubActivities.GithubListPRApprovals, activities.ListPRApprovalsRequest{
		Repo:     revision.Repo,
		PRNumber: d.PRNumber,
	}).Get(ctx, &listPRApprovalsResponse)
	if err != nil {
		return err
	}

	// Filter for approvals on stale commits
	var staleApprovals []int64
	staleCommits := findStaleCommits(listCommitsResponse.Commits, revision.Revision)
	for _, approval := range listPRApprovalsResponse.Approvals {
		for _, staleCommit := range staleCommits {
			if approval.GetCommitID() == staleCommit.GetSHA() {
				staleApprovals = append(staleApprovals, approval.GetID())
				break
			}
		}
	}

	// Dismiss each stale approval
	for _, staleApproval := range staleApprovals {
		err = workflow.ExecuteActivity(ctx, d.githubActivities.GithubDismiss, activities.DismissRequest{
			Repo:          revision.Repo,
			PRNumber:      d.PRNumber,
			ReviewID:      staleApproval,
			DismissReason: dismissReason,
		}).Get(ctx, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func findStaleCommits(commits []*github.RepositoryCommit, currentRevision string) []*github.RepositoryCommit {
	// we sort by timestamp here to avoid multiple calls to compare commits
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