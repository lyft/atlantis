package raw_test

import (
	"encoding/json"
	"testing"

	"github.com/runatlantis/atlantis/server/config/raw"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

func TestGithub_Unmarshal(t *testing.T) {
	t.Run("yaml", func(t *testing.T) {
		rawYaml := `
gateway_app_installation_id: 567
temporal_app_installation_id: 1234
`

		var result raw.Github

		err := yaml.UnmarshalStrict([]byte(rawYaml), &result)
		assert.NoError(t, err)
	})

	t.Run("json", func(t *testing.T) {
		rawJSON := `
{
	"temporal_app_installation_id": 1234,
	"gateway_app_installation_id": 567
}		
`

		var result raw.Github

		err := json.Unmarshal([]byte(rawJSON), &result)
		assert.NoError(t, err)
	})
}

func TestGithub_Validate_Success(t *testing.T) {
	cases := []struct {
		description string
		subject     *raw.Github
	}{
		{
			description: "success",
			subject: &raw.Github{
				GatewayAppInstallationID:  int64(123),
				TemporalAppInstallationID: int64(567),
			},
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			assert.NoError(t, c.subject.Validate())
		})
	}
}

func TestGithub_Validate_Error(t *testing.T) {
	cases := []struct {
		description string
		subject     raw.Github
	}{
		{
			description: "missing gateway id",
			subject: raw.Github{
				GatewayAppInstallationID: int64(123),
			},
		},
		{
			description: "missing temporal id",
			subject: raw.Github{
				TemporalAppInstallationID: int64(456),
			},
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			assert.Error(t, c.subject.Validate())
		})
	}
}
