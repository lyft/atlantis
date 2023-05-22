package requirement

import (
	"context"
	"testing"

	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/neptune/template"
	"github.com/stretchr/testify/assert"
)

type testErrGenerator[T any] struct {
	err ForbiddenError
}

func (g testErrGenerator[T]) GenerateForbiddenError(ctx context.Context, key template.Key, repo models.Repo, data T, msg string, format ...any) ForbiddenError {
	return g.err
}

func TestBranch_OnDefaultBranch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		expectedCriteria := Criteria{
			Repo:   models.Repo{Name: "hi", DefaultBranch: "main"},
			Branch: "main",
		}

		globalCfg := valid.NewGlobalCfg("")
		subject := &branchRestriction{
			cfg:            globalCfg,
			errorGenerator: testErrGenerator[template.BranchForbiddenData]{},
		}

		err := subject.Check(context.Background(), expectedCriteria)
		assert.NoError(t, err)
	})
	t.Run("error", func(t *testing.T) {
		expectedCriteria := Criteria{
			Repo:   models.Repo{Name: "hi", DefaultBranch: "main"},
			Branch: "notmain",
		}

		expectedError := ForbiddenError{details: "hi"}

		globalCfg := valid.NewGlobalCfg("")
		subject := &branchRestriction{
			cfg: globalCfg,
			errorGenerator: testErrGenerator[template.BranchForbiddenData]{
				err: expectedError,
			},
		}

		err := subject.Check(context.Background(), expectedCriteria)
		assert.EqualError(t, err, expectedError.Error())
	})
}

func TestBranch_NoRestriction(t *testing.T) {
	expectedCriteria := Criteria{
		Repo:   models.Repo{Name: "hi", DefaultBranch: "main"},
		Branch: "notmain",
	}

	globalCfg := valid.NewGlobalCfg("")
	globalCfg.Repos[0].ApplySettings.BranchRestriction = valid.NoBranchRestriction
	subject := &branchRestriction{
		cfg:            globalCfg,
		errorGenerator: testErrGenerator[template.BranchForbiddenData]{},
	}

	err := subject.Check(context.Background(), expectedCriteria)
	assert.NoError(t, err)
}
