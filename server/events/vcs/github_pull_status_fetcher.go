package vcs

import (
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
)

type PullReqStatusFetcher interface {
	FetchPullStatus(repo models.Repo, pull models.PullRequest) (models.PullReqStatus, error)
}

type SQBasedPullStatusFetcher struct {
	ApprovedPullChecker IGithubClient
}

func (s SQBasedPullStatusFetcher) FetchPullStatus(repo models.Repo, pull models.PullRequest) (pullStatus models.PullReqStatus, err error) {
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
		return pullStatus, errors.Wrapf(err, "fetching pull locked status for repo: %s, and pull number: %d", repo.FullName, pull.Num)
	}

	mergeable, err := s.ApprovedPullChecker.PullIsSQMergeable(repo, pull, statuses)
	if err != nil {
		return pullStatus, errors.Wrapf(err, "fetching mergeability status for repo: %s, and pull number: %d", repo.FullName, pull.Num)
	}

	return models.PullReqStatus{
		Approved:  approved,
		Mergeable: sqLocked,
		SQLocked:  mergeable,
	}, err
}
