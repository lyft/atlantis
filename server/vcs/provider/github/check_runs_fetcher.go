package github

import (
	"context"
	gh "github.com/google/go-github/v45/github"
	"github.com/runatlantis/atlantis/server/events/models"
	"regexp"
)

const (
	CompletedStatus  = "completed"
	FailedConclusion = "failed"
)

var checkRunRegex = regexp.MustCompile("atlantis/policy_check: .*")

type CheckRunsFetcher struct {
	GithubListIterator *ListIterator
	AppID              int64
}

func (r *CheckRunsFetcher) ListFailedPolicyCheckRuns(ctx context.Context, installationToken int64, repo models.Repo, ref string) ([]string, error) {
	run := func(ctx context.Context, client *gh.Client, nextPage int) (interface{}, *gh.Response, error) {
		listOptions := gh.ListCheckRunsOptions{
			Status: gh.String(CompletedStatus),
			AppID:  gh.Int64(r.AppID),
			ListOptions: gh.ListOptions{
				PerPage: 100,
			},
		}
		listOptions.Page = nextPage
		return client.Checks.ListCheckRunsForRef(ctx, repo.Owner, repo.Name, ref, &listOptions)
	}

	process := func(i interface{}) []string {
		var failedPolicyCheckRuns []string
		checkRunResults := i.(gh.ListCheckRunsResults)
		for _, checkRun := range checkRunResults.CheckRuns {
			if checkRunRegex.MatchString(checkRun.GetName()) && checkRun.GetConclusion() == FailedConclusion {
				failedPolicyCheckRuns = append(failedPolicyCheckRuns, checkRun.GetName())
			}
		}
		return failedPolicyCheckRuns
	}

	return r.GithubListIterator.Iterate(ctx, installationToken, run, process)
}
