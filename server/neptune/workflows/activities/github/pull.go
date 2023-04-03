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
