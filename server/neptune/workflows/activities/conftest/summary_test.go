package conftest

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"gopkg.in/go-playground/assert.v1"
	"testing"
)

func TestNewValidateSummaryFromResults(t *testing.T) {
	testResults := []activities.ValidationResult{
		{
			PolicySet: activities.PolicySet{
				Name: "policy1",
			},
			Status: activities.Fail,
		},
		{
			PolicySet: activities.PolicySet{
				Name: "policy2",
			},
			Status: activities.Success,
		},
	}
	summary := NewValidateSummaryFromResults(testResults)
	assert.Equal(t, summary.Failures, []string{"policy1"})
	assert.Equal(t, summary.Successes, []string{"policy2"})
}

func TestValidateSummary_IsEmpty(t *testing.T) {
	summary := ValidateSummary{}
	assert.Equal(t, summary.IsEmpty(), true)
	summary = ValidateSummary{
		Failures:  []string{"policy1"},
		Successes: []string{"policy2"},
	}
	assert.Equal(t, summary.IsEmpty(), false)
}

func TestValidateSummary_String(t *testing.T) {
	summary := ValidateSummary{}
	assert.Equal(t, summary.String(), "No policies exist in this run.")
	summary = ValidateSummary{
		Failures:  []string{},
		Successes: []string{"policy2"},
	}
	assert.Equal(t, summary.String(), "Successful policies: policy2\n\nFailing policies: None\n")
	summary = ValidateSummary{
		Failures:  []string{"policy1"},
		Successes: []string{"policy2"},
	}
	assert.Equal(t, summary.String(), "Successful policies: policy2\n\nFailing policies: policy1\n")
}
