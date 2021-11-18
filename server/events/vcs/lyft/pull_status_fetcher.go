package vcs

import (
	"github.com/google/go-github/v31/github"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
)

// Redefining this interface here to ensure we don't have a cyclic dependency with the surrounding package
type pullRequestStatusFetcher interface {
	FetchPullStatus(repo models.Repo, pull models.PullRequest) (models.PullReqStatus, error)
}

type pullClient interface {
	GetRepoStatuses(repo models.Repo, pull models.PullRequest) ([]*github.RepoStatus, error)
	PullIsSQMergeable(repo models.Repo, pull models.PullRequest, statuses []*github.RepoStatus) (bool, error)
	PullIsSQLocked(baseRepo models.Repo, pull models.PullRequest, statuses []*github.RepoStatus) (bool, error)
}

type SQBasedPullStatusFetcher struct {
	Delegate pullRequestStatusFetcher
	Client   pullClient
}

func NewSQBasedPullStatusFetcher(delegate pullRequestStatusFetcher, client pullClient) *SQBasedPullStatusFetcher {
	return &SQBasedPullStatusFetcher{
		Delegate: delegate,
		Client:   client,
	}
}

func (s *SQBasedPullStatusFetcher) FetchPullStatus(repo models.Repo, pull models.PullRequest) (models.PullReqStatus, error) {

	pullStatus, err := s.Delegate.FetchPullStatus(repo, pull)
	if err != nil {
		return pullStatus, err
	}

	statuses, err := s.Client.GetRepoStatuses(repo, pull)
	if err != nil {
		return pullStatus, errors.Wrapf(err, "fetching repo statuses for repo: %s, and pull number: %d", repo.FullName, pull.Num)
	}

	sqLocked, err := s.Client.PullIsSQLocked(repo, pull, statuses)
	if err != nil {
		return pullStatus, errors.Wrapf(err, "fetching pull locked status for repo: %s, and pull number: %d", repo.FullName, pull.Num)
	}

	mergeable, err := s.Client.PullIsSQMergeable(repo, pull, statuses)
	if err != nil {
		return pullStatus, errors.Wrapf(err, "fetching mergeability status for repo: %s, and pull number: %d", repo.FullName, pull.Num)
	}

	pullStatus.Mergeable = pullStatus.Mergeable || mergeable
	pullStatus.SQLocked = sqLocked

	return pullStatus, nil
}
