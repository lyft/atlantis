package requirement

import (
	"context"
	"testing"

	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/stretchr/testify/assert"
)

func TestPull(t *testing.T) {
	t.Run("no pull specified", func(t *testing.T) {
		expectedCriteria := Criteria{
			Repo: models.Repo{Name: "hi", DefaultBranch: "main"},
		}

		subject := pull{}

		err := subject.Check(context.Background(), expectedCriteria)
		assert.NoError(t, err)
	})

	t.Run("forked pull", func(t *testing.T) {
		expectedCriteria := Criteria{
			Repo: models.Repo{Name: "hi", DefaultBranch: "main"},
			OptionalPull: &models.PullRequest{
				BaseRepo: models.Repo{Owner: "lyft"},
				HeadRepo: models.Repo{Owner: "notlyft"},
			},
		}

		subject := pull{}

		err := subject.Check(context.Background(), expectedCriteria)
		var forbidden ForbiddenError
		assert.ErrorAs(t, err, &forbidden)

		assert.Equal(t, "pull request cannot be from a fork", forbidden.Error())
	})

	t.Run("closed pull", func(t *testing.T) {
		expectedCriteria := Criteria{
			Repo: models.Repo{Name: "hi", DefaultBranch: "main"},
			OptionalPull: &models.PullRequest{
				BaseRepo: models.Repo{Owner: "lyft"},
				HeadRepo: models.Repo{Owner: "lyft"},
				State:    models.ClosedPullState,
			},
		}

		subject := pull{}

		err := subject.Check(context.Background(), expectedCriteria)
		var forbidden ForbiddenError
		assert.ErrorAs(t, err, &forbidden)

		assert.Equal(t, "deploy cannot be executed on a closed pull request", forbidden.Error())
	})

	t.Run("valid pull", func(t *testing.T) {
		expectedCriteria := Criteria{
			Repo: models.Repo{Name: "hi", DefaultBranch: "main"},
			OptionalPull: &models.PullRequest{
				BaseRepo: models.Repo{Owner: "lyft"},
				HeadRepo: models.Repo{Owner: "lyft"},
				State:    models.OpenPullState,
			},
		}

		subject := pull{}

		err := subject.Check(context.Background(), expectedCriteria)
		assert.NoError(t, err)
	})
}
