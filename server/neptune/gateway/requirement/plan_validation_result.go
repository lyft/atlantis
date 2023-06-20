package requirement

import (
	"context"
	"fmt"
	"regexp"

	"github.com/google/go-github/v45/github"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/models"
	"github.com/runatlantis/atlantis/server/neptune/template"
)

var checkRunRegex = regexp.MustCompile("atlantis/policy_check: (?P<name>.+)")

type checkRunFetcher interface {
	ListFailedPolicyCheckRuns(ctx context.Context, installationToken int64, repo models.Repo, ref string) ([]*github.CheckRun, error)
}

type planValidationResult struct {
	cfg            valid.GlobalCfg
	errorGenerator errGenerator[template.PlanValidationSuccessData]
	fetcher        checkRunFetcher
}

func (a planValidationResult) Check(ctx context.Context, criteria Criteria) error {
	match := a.cfg.MatchingRepo(criteria.Repo.ID())

	shouldCheck := match.ApplySettings.ContainsPRRequirement(valid.PoliciesPassedApplyReq)

	if !shouldCheck || criteria.OptionalPull == nil {
		return nil
	}

	checkRuns, err := a.fetcher.ListFailedPolicyCheckRuns(ctx, criteria.InstallationToken, criteria.Repo, criteria.OptionalPull.HeadCommit)
	if err != nil {
		return errors.Wrap(err, "listing failed policy check runs")
	}

	rootsByName := make(map[string]*valid.MergedProjectCfg)
	for _, r := range criteria.Roots {
		rootsByName[r.Name] = r
	}

	var forbiddenCheckRuns []template.CheckRun
	// if failing check run is a root that we are intending to apply, let's forbid it.
	for _, c := range checkRuns {
		matches := checkRunRegex.FindStringSubmatch(c.GetName())
		if len(matches) != 2 {
			return fmt.Errorf("unable to determine root name from check run: %s", c)
		}
		rootName := matches[checkRunRegex.SubexpIndex("name")]
		if _, ok := rootsByName[rootName]; ok {
			forbiddenCheckRuns = append(forbiddenCheckRuns, template.CheckRun{
				Name: c.GetName(),
				URL:  c.GetDetailsURL(),
			})
		}
	}

	if len(forbiddenCheckRuns) > 0 {
		return a.errorGenerator.GenerateForbiddenError(
			ctx,
			template.PlanValidationSuccess,
			criteria.Repo,
			template.PlanValidationSuccessData{CheckRuns: forbiddenCheckRuns},
			"plan validation steps must be successful",
		)
	}

	return nil
}
