package terraform_test

import (
	"testing"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/stretchr/testify/assert"
)

func TestNewFailure(t *testing.T) {
	output := "╷\n" +
		"│ Error: Failed to query available provider packages\n" +
		"│\n" +
		"│ Could not retrieve the list of available versions for provider\n" +
		"│ hashicorp/aws: no available releases match the given constraints >= 6.0.0\n" +
		"╵\n"

	f := terraform.NewFailure(output)

	assert.False(t, f.IsEmpty())
	assert.Equal(t, []string{
		"Error: Failed to query available provider packages\n" +
			"\n" +
			"Could not retrieve the list of available versions for provider\n" +
			"hashicorp/aws: no available releases match the given constraints >= 6.0.0",
	}, f.Errors)
	if assert.NotNil(t, f.Hint) {
		assert.Equal(t, "provider-version-conflict", f.Hint.Rule)
	}
}

func TestNewFailure_unrecognizedOutput(t *testing.T) {
	assert.True(t, terraform.NewFailure("exit status 1").IsEmpty())
}
