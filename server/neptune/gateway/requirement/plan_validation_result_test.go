package requirement

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-github/v45/github"
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/models"
	"github.com/runatlantis/atlantis/server/neptune/template"
	"github.com/stretchr/testify/assert"
)

type testCheckRunFetcher struct {
	checks []*github.CheckRun
	err    error
}

func (f *testCheckRunFetcher) ListFailedPolicyCheckRuns(ctx context.Context, installationToken int64, repo models.Repo, ref string) ([]*github.CheckRun, error) {
	return f.checks, f.err
}

func TestPlanValidationResult(t *testing.T) {
	t.Run("requirement not specified", func(t *testing.T) {
		expectedCriteria := Criteria{
			Repo:         models.Repo{Name: "hi", DefaultBranch: "main"},
			Branch:       "main",
			OptionalPull: &models.PullRequest{},
		}

		globalCfg := valid.NewGlobalCfg("")
		subject := &planValidationResult{
			cfg:            globalCfg,
			errorGenerator: testErrGenerator[template.PlanValidationSuccessData]{},
		}

		err := subject.Check(context.Background(), expectedCriteria)
		assert.NoError(t, err)
	})

	t.Run("pull not specified", func(t *testing.T) {
		expectedCriteria := Criteria{
			Repo:   models.Repo{Name: "hi", DefaultBranch: "main"},
			Branch: "main",
		}

		globalCfg := valid.NewGlobalCfg("")
		globalCfg.Repos[0].ApplySettings.PRRequirements = []string{valid.PoliciesPassedApplyReq}
		subject := &planValidationResult{
			cfg:            globalCfg,
			errorGenerator: testErrGenerator[template.PlanValidationSuccessData]{},
		}

		err := subject.Check(context.Background(), expectedCriteria)
		assert.NoError(t, err)
	})

	t.Run("failed checks", func(t *testing.T) {
		root := "root1"
		detailsURL := "www.details.com"
		expectedCriteria := Criteria{
			Repo:         models.Repo{Name: "hi", DefaultBranch: "main"},
			Branch:       "main",
			OptionalPull: &models.PullRequest{Num: 1},
			Roots: []*valid.MergedProjectCfg{
				{
					Name: root,
				},
			},
		}

		globalCfg := valid.NewGlobalCfg("")
		globalCfg.Repos[0].ApplySettings.PRRequirements = []string{valid.PoliciesPassedApplyReq}

		expectedError := ForbiddenError{details: "hi"}

		subject := &planValidationResult{
			cfg:            globalCfg,
			errorGenerator: testErrGenerator[template.PlanValidationSuccessData]{err: expectedError},
			fetcher: &testCheckRunFetcher{
				checks: []*github.CheckRun{
					{
						Name:       github.String(fmt.Sprintf("atlantis/policy_check: %s", root)),
						DetailsURL: github.String(detailsURL),
					},
				},
			},
		}

		err := subject.Check(context.Background(), expectedCriteria)

		var target ForbiddenError
		assert.ErrorAs(t, err, &target)
		assert.Equal(t, expectedError, target)
	})

	t.Run("no failing checks for root1", func(t *testing.T) {
		root := "root1"
		detailsURL := "www.details.com"
		expectedCriteria := Criteria{
			Repo:         models.Repo{Name: "hi", DefaultBranch: "main"},
			Branch:       "main",
			OptionalPull: &models.PullRequest{Num: 1},
			Roots: []*valid.MergedProjectCfg{
				{
					Name: root,
				},
			},
		}

		globalCfg := valid.NewGlobalCfg("")
		globalCfg.Repos[0].ApplySettings.PRRequirements = []string{valid.PoliciesPassedApplyReq}

		subject := &planValidationResult{
			cfg:            globalCfg,
			errorGenerator: testErrGenerator[template.PlanValidationSuccessData]{},
			fetcher: &testCheckRunFetcher{
				checks: []*github.CheckRun{
					{
						Name:       github.String("atlantis/policy_check: root2"),
						DetailsURL: github.String(detailsURL),
					},
				},
			},
		}

		err := subject.Check(context.Background(), expectedCriteria)
		assert.NoError(t, err)
	})
}
