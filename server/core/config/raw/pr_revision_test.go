package raw_test

import (
	"encoding/json"
	"testing"

	"github.com/runatlantis/atlantis/server/core/config/raw"
	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

func TestPRRevision_Unmarshal(t *testing.T) {
	t.Run("yaml", func(t *testing.T) {

		rawYaml := `
username: test-user
password_env: TEST_ENV_PASSWORD
url: https://test-url.com
`
		var result raw.PRRevision

		err := yaml.UnmarshalStrict([]byte(rawYaml), &result)
		assert.NoError(t, err)
	})

	t.Run("json", func(t *testing.T) {
		rawJSON := `
	{
		"username": "jobs",
		"password_env": "TEST_ENV_PASSWORD",
		"url": "https://test-url.com"
	}
	`
		var result raw.PRRevision

		err := json.Unmarshal([]byte(rawJSON), &result)
		assert.NoError(t, err)
	})

}

func TestPRRevision_Validate_Success(t *testing.T) {
	prRevision := &raw.PRRevision{
		Username:    "test-username",
		PasswordEnv: "TEST_PASSWORD_ENV",
		URL:         "https://test-url.com",
	}
	assert.NoError(t, prRevision.Validate())
}

func TestPRRevision_Validate_Error(t *testing.T) {
	cases := []struct {
		description string
		subject     raw.PRRevision
	}{
		{
			description: "missing username",
			subject: raw.PRRevision{
				PasswordEnv: "TEST_PASSWORD_ENV",
				URL:         "https://tes-url.com",
			},
		},
		{
			description: "missing password_env",
			subject: raw.PRRevision{
				Username: "test-username",
				URL:      "https://tes-url.com",
			},
		},
		{
			description: "missing url",
			subject: raw.PRRevision{
				Username:    "test-username",
				PasswordEnv: "TEST_PASSWORD_ENV",
			},
		},
		{
			description: "invalid url",
			subject: raw.PRRevision{
				Username:    "test-username",
				PasswordEnv: "TEST_PASSWORD_ENV",
				URL:         "tes-url&^*&.com",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			assert.Error(t, c.subject.Validate())
		})
	}
}
