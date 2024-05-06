package github

import (
	"time"
)

// Add more fields as necessary
type PullRequest struct {
	Number    int
	UpdatedAt time.Time
	// Whether the PR has a label of "automated" or not, useful for identifying refactorator PRs
	IsAutomatedPR bool
}

type PullRequestState string

const OpenPullRequest PullRequestState = "open"

type SortKey string

const Updated SortKey = "updated"

type Order string

const Descending Order = "desc"
