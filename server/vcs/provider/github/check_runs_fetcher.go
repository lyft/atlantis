package github

import (
	"context"
	"fmt"
	gh "github.com/google/go-github/v45/github"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
	"net/http"
	"regexp"
)

const (
	CompletedStatus  = "completed"
	FailedConclusion = "failed"
)

var checkRunRegex = regexp.MustCompile("atlantis/policy_check: .*")

type CheckRunsFetcher struct {
	ClientCreator githubapp.ClientCreator
	AppID         int64
}

func (r *CheckRunsFetcher) ListFailedPolicyCheckRuns(ctx context.Context, installationToken int64, repo models.Repo, ref string) ([]string, error) {
	client, err := r.ClientCreator.NewInstallationClient(installationToken)
	if err != nil {
		return nil, errors.Wrap(err, "creating installation client")
	}

	var failedPolicyCheckRuns []string
	nextPage := 0
	for {
		listOptions := gh.ListCheckRunsOptions{
			Status: gh.String(CompletedStatus),
			AppID:  gh.Int64(r.AppID),
			ListOptions: gh.ListOptions{
				PerPage: 100,
			},
		}
		listOptions.Page = nextPage

		checkRunResults, resp, err := client.Checks.ListCheckRunsForRef(ctx, repo.Owner, repo.Name, ref, &listOptions)
		if err != nil {
			return nil, errors.Wrap(err, "error fetching check runs for ref")
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("not ok status fetching check runs for ref: %s", resp.Status)
		}
		for _, checkRun := range checkRunResults.CheckRuns {
			if checkRunRegex.MatchString(checkRun.GetName()) && checkRun.GetConclusion() == FailedConclusion {
				failedPolicyCheckRuns = append(failedPolicyCheckRuns, checkRun.GetName())
			}
		}
		if resp.NextPage == 0 {
			break
		}
		nextPage = resp.NextPage
	}
	return failedPolicyCheckRuns, nil
}
