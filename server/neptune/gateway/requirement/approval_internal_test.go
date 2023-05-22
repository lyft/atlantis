package requirement

import (
	"context"
	"testing"

	"github.com/google/go-github/v45/github"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/neptune/template"
	"github.com/stretchr/testify/assert"
)

type testReviewFetcher struct {
	reviews []*github.PullRequestReview
	err     error
}

func (t testReviewFetcher) ListApprovalReviews(ctx context.Context, installationToken int64, repo models.Repo, prNum int) ([]*github.PullRequestReview, error) {
	return t.reviews, t.err
}

func TestApproval(t *testing.T) {
	t.Run("requirement not specified", func(t *testing.T) {
		expectedCriteria := Criteria{
			Repo:         models.Repo{Name: "hi", DefaultBranch: "main"},
			Branch:       "main",
			OptionalPull: &models.PullRequest{},
		}

		globalCfg := valid.NewGlobalCfg("")
		subject := &approval{
			cfg:            globalCfg,
			errorGenerator: testErrGenerator[template.Input]{},
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
		globalCfg.Repos[0].ApplySettings.PRRequirements = []string{valid.ApprovedApplyReq}
		subject := &approval{
			cfg:            globalCfg,
			errorGenerator: testErrGenerator[template.Input]{},
		}

		err := subject.Check(context.Background(), expectedCriteria)
		assert.NoError(t, err)
	})

	t.Run("no approvals", func(t *testing.T) {
		expectedCriteria := Criteria{
			Repo:         models.Repo{Name: "hi", DefaultBranch: "main"},
			Branch:       "main",
			OptionalPull: &models.PullRequest{Num: 1},
		}

		globalCfg := valid.NewGlobalCfg("")
		globalCfg.Repos[0].ApplySettings.PRRequirements = []string{valid.ApprovedApplyReq}

		expectedError := ForbiddenError{details: "hi"}

		subject := &approval{
			cfg:            globalCfg,
			errorGenerator: testErrGenerator[template.Input]{err: expectedError},
			fetcher:        testReviewFetcher{},
		}

		err := subject.Check(context.Background(), expectedCriteria)

		var target ForbiddenError
		assert.ErrorAs(t, err, &target)
		assert.Equal(t, expectedError, target)
	})

	t.Run("approved", func(t *testing.T) {
		expectedCriteria := Criteria{
			Repo:         models.Repo{Name: "hi", DefaultBranch: "main"},
			Branch:       "main",
			OptionalPull: &models.PullRequest{Num: 1},
		}

		globalCfg := valid.NewGlobalCfg("")
		globalCfg.Repos[0].ApplySettings.PRRequirements = []string{valid.ApprovedApplyReq}

		expectedError := ForbiddenError{details: "hi"}

		subject := &approval{
			cfg:            globalCfg,
			errorGenerator: testErrGenerator[template.Input]{err: expectedError},
			fetcher: testReviewFetcher{
				reviews: []*github.PullRequestReview{
					{
						ID: github.Int64(1),
					},
				},
			},
		}

		err := subject.Check(context.Background(), expectedCriteria)
		assert.NoError(t, err)
	})
}
