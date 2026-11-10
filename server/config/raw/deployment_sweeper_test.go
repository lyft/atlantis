package raw_test

import (
	"testing"
	"time"

	"github.com/runatlantis/atlantis/server/config/raw"
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestDeploymentSweeper_ToValid(t *testing.T) {
	cases := []struct {
		description string
		input       string
		expected    valid.DeploymentSweeper
	}{
		{
			description: "left out is a dry run with the default max age",
			input:       "{}",
			expected:    valid.DeploymentSweeper{},
		},
		{
			description: "delete with a max age",
			input:       "max_age: 360h\ndelete: true",
			expected:    valid.DeploymentSweeper{MaxAge: 360 * time.Hour, Delete: true},
		},
		{
			description: "max age only stays a dry run",
			input:       "max_age: 48h",
			expected:    valid.DeploymentSweeper{MaxAge: 48 * time.Hour},
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			var cfg raw.DeploymentSweeper
			require.NoError(t, yaml.Unmarshal([]byte(c.input), &cfg))
			require.NoError(t, cfg.Validate())
			assert.Equal(t, c.expected, cfg.ToValid())
		})
	}
}

func TestDeploymentSweeper_ValidateRejectsBadMaxAge(t *testing.T) {
	for _, maxAge := range []string{"15d", "soon", "-"} {
		t.Run(maxAge, func(t *testing.T) {
			assert.Error(t, raw.DeploymentSweeper{MaxAge: maxAge}.Validate())
		})
	}
}
