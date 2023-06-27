package conftest

import (
	"fmt"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"strings"
)

type ValidateSummary struct {
	Failures  []string
	Successes []string
}

func NewValidateSummaryFromResults(results []activities.ValidationResult) ValidateSummary {
	if len(results) == 0 {
		return ValidateSummary{}
	}

	var failures []string
	var successes []string
	for _, result := range results {
		summary := result.PolicySet.Name
		switch result.Status {
		case activities.Fail:
			failures = append(failures, summary)
		case activities.Success:
			successes = append(successes, summary)
		}
	}

	return ValidateSummary{
		Failures:  failures,
		Successes: successes,
	}
}

func (s ValidateSummary) IsEmpty() bool {
	return len(s.Successes) == 0 && len(s.Failures) == 0
}

func (s ValidateSummary) String() string {
	if s.IsEmpty() {
		return "No policies exist in this run."
	}
	successes := strings.Join(s.Successes, ", ")
	if successes == "" {
		successes = "None"
	}
	failures := strings.Join(s.Failures, ", ")
	if failures == "" {
		failures = "None"
	}

	return fmt.Sprintf(
		"Successful policies: %s\n\n"+
			"Failing policies: %s\n",
		successes, failures)
}
