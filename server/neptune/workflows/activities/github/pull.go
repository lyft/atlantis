package github

import (
	"time"
)

// Add more fields as necessary
type PullRequest struct {
	Number    int
	UpdatedAt time.Time
}

type PullRequestState string

const OpenPullRequest PullRequestState = "open"

type SortKey string

const Updated SortKey = "updated"

type Order string

const Descending Order = "desc"
