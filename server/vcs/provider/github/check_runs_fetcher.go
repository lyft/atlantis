package github

import (
	"context"
	"regexp"

	gh "github.com/google/go-github/v45/github"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/models"
)

const (
	CompletedStatus  = "completed"
	FailedConclusion = "failure"
)

var checkRunRegex = regexp.MustCompile("atlantis/policy_check.*")

type CheckRunsFetcher struct {
	ClientCreator githubapp.ClientCreator
	AppID         int64
}

func (r *CheckRunsFetcher) ListFailedPolicyCheckRunNames(ctx context.Context, installationToken int64, repo models.Repo, ref string) ([]string, error) {
	runs, err := r.ListFailedPolicyCheckRuns(ctx, installationToken, repo, ref)
	if err != nil {
		return []string{}, err
	}

	var results []string
	for _, r := range runs {
		results = append(results, r.GetName())
	}

	return results, nil
}

func (r *CheckRunsFetcher) ListFailedPolicyCheckRuns(ctx context.Context, installationToken int64, repo models.Repo, ref string) ([]*gh.CheckRun, error) {
	client, err := r.ClientCreator.NewInstallationClient(installationToken)
	if err != nil {
		return nil, errors.Wrap(err, "creating installation client")
	}
	run := func(ctx context.Context, nextPage int) ([]*gh.CheckRun, *gh.Response, error) {
		listOptions := gh.ListCheckRunsOptions{
			Status: gh.String(CompletedStatus),
			AppID:  gh.Int64(r.AppID),
			ListOptions: gh.ListOptions{
				PerPage: 100,
			},
		}
		listOptions.Page = nextPage
		checkRunResults, resp, err := client.Checks.ListCheckRunsForRef(ctx, repo.Owner, repo.Name, ref, &listOptions)
		if checkRunResults != nil {
			return checkRunResults.CheckRuns, resp, err
		}
		return nil, resp, errors.Wrap(err, "unable to retrieve check runs from GH check run results")
	}

	checkRuns, err := Iterate(ctx, run)
	if err != nil {
		return nil, errors.Wrap(err, "iterating through entries")
	}
	var failedPolicyCheckRuns []*gh.CheckRun
	for _, checkRun := range checkRuns {
		if checkRunRegex.MatchString(checkRun.GetName()) && checkRun.GetConclusion() == FailedConclusion {
			failedPolicyCheckRuns = append(failedPolicyCheckRuns, checkRun)
		}
	}
	return failedPolicyCheckRuns, nil
}
