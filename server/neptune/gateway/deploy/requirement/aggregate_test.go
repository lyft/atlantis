package requirement_test

import (
	"context"
	"testing"

	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/neptune/gateway/deploy/requirement"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"github.com/stretchr/testify/assert"
)

type noopRequirement struct {
	expectedT         *testing.T
	expectedCriterita requirement.Criteria
	expectedErr       error

	called bool
}

func (r *noopRequirement) Check(ctx context.Context, criteria requirement.Criteria) error {
	assert.Equal(r.expectedT, r.expectedCriterita, criteria)

	r.called = true
	return r.expectedErr
}

func TestAggregate_Success(t *testing.T) {
	expectedCriteria := requirement.Criteria{
		Repo: models.Repo{Name: "hi"},
	}
	overrideable := &noopRequirement{
		expectedT:         t,
		expectedCriterita: expectedCriteria,
		expectedErr:       nil,
	}

	nonOverrideable := &noopRequirement{
		expectedT:         t,
		expectedCriterita: expectedCriteria,
		expectedErr:       nil,
	}

	subject := requirement.NewAggregateWithRequirements([]requirement.Requirement{overrideable}, []requirement.Requirement{nonOverrideable})

	err := subject.Check(context.Background(), expectedCriteria)
	assert.NoError(t, err)
	assert.True(t, overrideable.called)
	assert.True(t, nonOverrideable.called)
}

func TestAggregate_ForceTrigger(t *testing.T) {
	expectedCriteria := requirement.Criteria{
		Repo: models.Repo{Name: "hi"},
		TriggerInfo: workflows.DeployTriggerInfo{
			Force: true,
		},
	}
	overrideable := &noopRequirement{
		expectedT:         t,
		expectedCriterita: expectedCriteria,
		expectedErr:       nil,
	}

	nonOverrideable := &noopRequirement{
		expectedT:         t,
		expectedCriterita: expectedCriteria,
		expectedErr:       nil,
	}

	subject := requirement.NewAggregateWithRequirements([]requirement.Requirement{overrideable}, []requirement.Requirement{nonOverrideable})

	err := subject.Check(context.Background(), expectedCriteria)
	assert.NoError(t, err)
	assert.False(t, overrideable.called)
	assert.True(t, nonOverrideable.called)
}

func TestAggregate_OverrideableError(t *testing.T) {
	expectedCriteria := requirement.Criteria{
		Repo: models.Repo{Name: "hi"},
	}
	overrideable := &noopRequirement{
		expectedT:         t,
		expectedCriterita: expectedCriteria,
		expectedErr:       assert.AnError,
	}

	nonOverrideable := &noopRequirement{
		expectedT:         t,
		expectedCriterita: expectedCriteria,
		expectedErr:       nil,
	}

	subject := requirement.NewAggregateWithRequirements([]requirement.Requirement{overrideable}, []requirement.Requirement{nonOverrideable})

	err := subject.Check(context.Background(), expectedCriteria)
	assert.Error(t, err)
	assert.True(t, overrideable.called)
	assert.True(t, nonOverrideable.called)
}

func TestAggregate_NonOverrideableError(t *testing.T) {
	expectedCriteria := requirement.Criteria{
		Repo: models.Repo{Name: "hi"},
	}
	overrideable := &noopRequirement{
		expectedT:         t,
		expectedCriterita: expectedCriteria,
		expectedErr:       nil,
	}

	nonOverrideable := &noopRequirement{
		expectedT:         t,
		expectedCriterita: expectedCriteria,
		expectedErr:       assert.AnError,
	}

	subject := requirement.NewAggregateWithRequirements([]requirement.Requirement{overrideable}, []requirement.Requirement{nonOverrideable})

	err := subject.Check(context.Background(), expectedCriteria)
	assert.Error(t, err)
	assert.False(t, overrideable.called)
	assert.True(t, nonOverrideable.called)
}
