package lyft

import (
	"github.com/google/go-github/v31/github"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
)


type StatusViolationChecker struct {
	violationDeterminer StatusViolationDeterminer
	requiredStatusSet RequiredStatusSet
}

type StatusViolationDeterminer interface {
	isViolation(status *github.RepoStatus) bool
}

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

// type GithubClient struct {
// 	client *github.Client
// }

// func NewGithubClient(client *github.Client) *GithubClient {
// 	return &GithubClient{
// 		client: client,
// 	}
// }

// // PullIsMergeable returns true if the pull request is mergeable.
// func (g *GithubClient) PullIsMergeable(repo models.Repo, pull models.PullRequest) (bool, error) {
// 	githubPR, err := g.GetPullRequest(repo, pull.Num)
// 	if err != nil {
// 		return false, errors.Wrap(err, "getting pull request")
// 	}
// 	state := githubPR.GetMergeableState()
// 	// We map our mergeable check to when the GitHub merge button is clickable.
// 	// This corresponds to the following states:
// 	// clean: No conflicts, all requirements satisfied.
// 	//        Merging is allowed (green box).
// 	// unstable: Failing/pending commit status that is not part of the required
// 	//           status checks. Merging is allowed (yellow box).
// 	// has_hooks: GitHub Enterprise only, if a repo has custom pre-receive
// 	//            hooks. Merging is allowed (green box).
// 	// See: https://github.com/octokit/octokit.net/issues/1763
// 	if state != "clean" && state != "unstable" && state != "has_hooks" {

// 		if state != "blocked" {
// 			return false, nil
// 		}

// 		return g.getSupplementalMergeability(repo, pull)
// 	}
// 	return true, nil
// }