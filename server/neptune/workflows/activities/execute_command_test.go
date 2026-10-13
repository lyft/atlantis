package activities

import (
	"context"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/execute"
	"github.com/stretchr/testify/assert"
)

func TestExecuteCommand_TerraformVersionEnv(t *testing.T) {
	printVersion := execute.Step{RunCommand: "printf %s \"$ATLANTIS_TERRAFORM_VERSION\""}

	t.Run("uses the root's configured version", func(t *testing.T) {
		subject := &executeCommandActivities{DefaultTFVersion: version.Must(version.NewVersion("1.9.5"))}

		resp, err := subject.ExecuteCommand(context.Background(), ExecuteCommandRequest{
			Step:      printVersion,
			Path:      t.TempDir(),
			TfVersion: "1.10.5",
		})

		assert.NoError(t, err)
		assert.Equal(t, "1.10.5", resp.Output)
	})

	t.Run("falls back to the default version", func(t *testing.T) {
		subject := &executeCommandActivities{DefaultTFVersion: version.Must(version.NewVersion("1.10.5"))}

		resp, err := subject.ExecuteCommand(context.Background(), ExecuteCommandRequest{
			Step: printVersion,
			Path: t.TempDir(),
		})

		assert.NoError(t, err)
		assert.Equal(t, "1.10.5", resp.Output)
	})

	t.Run("an explicit env var overrides it", func(t *testing.T) {
		subject := &executeCommandActivities{DefaultTFVersion: version.Must(version.NewVersion("1.9.5"))}

		resp, err := subject.ExecuteCommand(context.Background(), ExecuteCommandRequest{
			Step:           printVersion,
			Path:           t.TempDir(),
			TfVersion:      "1.10.5",
			DynamicEnvVars: []EnvVar{{Name: "ATLANTIS_TERRAFORM_VERSION", Value: "1.13.1"}},
		})

		assert.NoError(t, err)
		assert.Equal(t, "1.13.1", resp.Output)
	})

	t.Run("does not set it when no version is known", func(t *testing.T) {
		subject := &executeCommandActivities{}

		resp, err := subject.ExecuteCommand(context.Background(), ExecuteCommandRequest{
			Step: printVersion,
			Path: t.TempDir(),
		})

		assert.NoError(t, err)
		assert.Equal(t, "", resp.Output)
	})

	t.Run("fails on an invalid configured version", func(t *testing.T) {
		subject := &executeCommandActivities{}

		_, err := subject.ExecuteCommand(context.Background(), ExecuteCommandRequest{
			Step:      printVersion,
			Path:      t.TempDir(),
			TfVersion: "not-a-version",
		})

		assert.Error(t, err)
	})
}
