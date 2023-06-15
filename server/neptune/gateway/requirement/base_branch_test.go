package requirement

import (
	"context"
	"regexp"
	"testing"

	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/models"
	"github.com/stretchr/testify/assert"
)

var regex = regexp.MustCompile(`abc`)

func TestBaseBranch(t *testing.T) {
	globalCfg := valid.GlobalCfg{
		Repos: []valid.Repo{
			{
				ID:          "/owner",
				BranchRegex: regex,
			},
		},
	}
	t.Run("success", func(t *testing.T) {
		subject := baseBranch{GlobalCfg: globalCfg}
		expectedCriteria := Criteria{
			OptionalPull: &models.PullRequest{
				BaseRepo:   models.Repo{FullName: "owner"},
				BaseBranch: "abc",
			},
		}
		err := subject.Check(context.Background(), expectedCriteria)
		assert.NoError(t, err)
	})
	t.Run("failure", func(t *testing.T) {
		subject := baseBranch{GlobalCfg: globalCfg}
		expectedCriteria := Criteria{
			OptionalPull: &models.PullRequest{
				BaseRepo:   models.Repo{FullName: "owner"},
				BaseBranch: "def",
			},
		}
		err := subject.Check(context.Background(), expectedCriteria)
		assert.Error(t, err)
	})
}
