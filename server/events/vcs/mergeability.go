package vcs

import (
	"github.com/google/go-github/v31/github"
	"github.com/runatlantis/atlantis/server/events/models"
)

type StatusViolationChecker struct {
	violationDeterminer StatusViolationDeterminer
	requiredStatusSet   RequiredStatusSet
}

type StatusViolationDeterminer interface {
	isViolation(status *github.RepoStatus) bool
}

type StatusStateViolationDeterminer struct {
	fallbackDeterminer StatusViolationDeterminer
}

func (d *StatusStateViolationDeterminer) isViolation(status *github.RepoStatus) bool {
	return status.GetState() != "success" &&
		d.fallbackDeterminer.isViolation(status)
}

type Status

type RequiredStatusSet interface {
	ContainsAll(status []*github.RepoStatus) bool
}

func (c *StatusViolationChecker) Check(repo models.Repo, pull models.PullRequest, statuses []*github.RepoStatus) (bool, error) {
	if !c.requiredStatusSet.ContainsAll(statuses) {
		return false, nil
	}

	for _, status := range statuses {
		if c.violationDeterminer.isViolation(status) {
			return false, nil
		}
	}

	return true, nil
}
