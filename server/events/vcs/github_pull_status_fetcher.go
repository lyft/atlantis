package vcs

import (
	"github.com/google/go-github/v31/github"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
)

type PullStatusFetcher interface {
	FetchPullStatus(repo models.Repo, pull models.PullRequest) (models.SQPullStatus, error)
}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_github_pull_status_fetcher.go PullApprovalChecker
type PullApprovalChecker interface {
	GetRepoStatuses(repo models.Repo, pull models.PullRequest) ([]*github.RepoStatus, error)
	PullIsApproved(repo models.Repo, pull models.PullRequest) (bool, error)
	PullIsSQMergeable(repo models.Repo, pull models.PullRequest, statuses []*github.RepoStatus) (bool, error)
	PullIsLocked(baseRepo models.Repo, pull models.PullRequest, statuses []*github.RepoStatus) (bool, error)
}

type SQBasedPullStatusFetcher struct {
	ApprovedPullChecker PullApprovalChecker
}

func (s *SQBasedPullStatusFetcher) FetchPullStatus(repo models.Repo, pull models.PullRequest) (models.SQPullStatus, error) {
	// Get Repo statuses.
	// Pass that that Pull Is Locked and Pull Is Mergeable (which forwards to getSubmitQueueMergeability)
	// Check pull is approved.

	pullStatus := models.SQPullStatus{
		Approved:  false,
		Mergeable: false,
		SQLocked:  false,
	}

	statuses, err := s.ApprovedPullChecker.GetRepoStatuses(repo, pull)
	if err != nil {
		return pullStatus, errors.Wrapf(err, "fetching repo statuses for repo: %s, and pull number: %d", repo.FullName, pull.Num)
	}

	approved, err := s.ApprovedPullChecker.PullIsApproved(repo, pull)
	if err != nil {
		return pullStatus, errors.Wrapf(err, "fetching pull approval status for repo: %s, and pull number: %d", repo.FullName, pull.Num)
	}

	sqLocked, err := s.ApprovedPullChecker.PullIsLocked(repo, pull, statuses)
	if err != nil {
		return pullStatus, errors.Wrapf(err, "fetching pull approval status for repo: %s, and pull number: %d", repo.FullName, pull.Num)
	}

	mergeable, err := s.ApprovedPullChecker.PullIsSQMergeable(repo, pull, statuses)
	if err != nil {
		return pullStatus, errors.Wrapf(err, "fetching mergeability status for repo: %s, and pull number: %d", repo.FullName, pull.Num)
	}

	return models.SQPullStatus{
		Approved:  approved,
		Mergeable: sqLocked,
		SQLocked:  mergeable,
	}, nil
}
