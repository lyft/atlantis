package requirement

import (
	"context"
	"testing"

	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/models"
	"github.com/runatlantis/atlantis/server/neptune/template"
	"github.com/stretchr/testify/assert"
)

type testFetcher struct {
	users []string
	err   error
}

func (f testFetcher) ListTeamMembers(ctx context.Context, installationToken int64, teamSlug string) ([]string, error) {
	return f.users, f.err
}

func TestTeam(t *testing.T) {
	t.Run("no configuration", func(t *testing.T) {
		expectedCriteria := Criteria{
			Repo: models.Repo{Name: "hi", DefaultBranch: "main"},
			User: models.User{Username: "nish"},
		}

		globalCfg := valid.NewGlobalCfg("")

		subject := team{
			cfg:     globalCfg,
			fetcher: testFetcher{},
		}

		err := subject.Check(context.Background(), expectedCriteria)
		assert.NoError(t, err)
	})

	t.Run("success", func(t *testing.T) {
		expectedCriteria := Criteria{
			Repo: models.Repo{Name: "hi", DefaultBranch: "main"},
			User: models.User{Username: "nish"},
		}

		globalCfg := valid.NewGlobalCfg("")
		globalCfg.Repos[0].ApplySettings.Team = "some-team"

		subject := team{
			cfg: globalCfg,
			fetcher: testFetcher{
				users: []string{
					"nish",
				},
			},
		}

		err := subject.Check(context.Background(), expectedCriteria)
		assert.NoError(t, err)
	})

	t.Run("forbidden", func(t *testing.T) {
		expectedCriteria := Criteria{
			Repo: models.Repo{Name: "hi", DefaultBranch: "main"},
			User: models.User{Username: "nish"},
		}

		globalCfg := valid.NewGlobalCfg("")
		globalCfg.Repos[0].ApplySettings.Team = "some-team"

		expectedErr := ForbiddenError{details: "hi"}
		subject := team{
			cfg: globalCfg,
			fetcher: testFetcher{
				users: []string{
					"test",
				},
			},
			errorGenerator: testErrGenerator[template.UserForbiddenData]{
				err: expectedErr,
			},
		}

		err := subject.Check(context.Background(), expectedCriteria)
		assert.EqualError(t, err, expectedErr.details)
	})
}
