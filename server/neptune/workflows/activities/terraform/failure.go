package terraform

import "github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform/failurehints"

// Failure describes why a Terraform command failed, built from its output.
type Failure struct {
	// Errors holds Terraform's error diagnostics, redacted and truncated.
	Errors []string
	// Hint explains the failure when a known rule matches.
	Hint *failurehints.Hint
}

// NewFailure builds a Failure from the output of a failed Terraform command.
func NewFailure(output string) Failure {
	f := Failure{Errors: failurehints.Errors(output)}
	if hint, ok := failurehints.Match(output, failurehints.DefaultRules()); ok {
		f.Hint = &hint
	}
	return f
}

func (f Failure) IsEmpty() bool {
	return len(f.Errors) == 0 && f.Hint == nil
}
