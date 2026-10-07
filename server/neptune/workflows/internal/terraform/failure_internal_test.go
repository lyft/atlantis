package terraform

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/temporal"
)

func TestTerraformFailure(t *testing.T) {
	failure := terraform.Failure{Errors: []string{"Error: Module not installed"}}

	t.Run("wrapped error with details", func(t *testing.T) {
		err := errors.Wrap(temporal.NewNonRetryableApplicationError("running plan command", "TerraformClientError", nil, failure), "running job")
		assert.Equal(t, failure, terraformFailure(err))
	})

	t.Run("error without details", func(t *testing.T) {
		err := temporal.NewNonRetryableApplicationError("running plan command", "TerraformClientError", nil)
		assert.True(t, terraformFailure(err).IsEmpty())
	})

	t.Run("other error", func(t *testing.T) {
		assert.True(t, terraformFailure(errors.New("timeout")).IsEmpty())
	})
}
