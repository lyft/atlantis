package raw_test

import (
	"encoding/json"
	"testing"

	"github.com/runatlantis/atlantis/server/config/raw"
	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

func TestPRRevision_Unmarshal(t *testing.T) {
	t.Run("yaml", func(t *testing.T) {
		rawYaml := `
url: https://test-url.com
basic_auth:
  username: test-user
  password: tes-password
default_task_queue:
  activities_per_second: 2.5
slow_task_queue:
  activities_per_second: 1.5
`
		var result raw.RevisionSetter

		err := yaml.UnmarshalStrict([]byte(rawYaml), &result)
		assert.NoError(t, err)
	})

	t.Run("json", func(t *testing.T) {
		rawJSON := `
	{
		"url": "https://test-url.com",
		"basic_auth": {
			"username": "test-user",
			"password": "test-password"
		},
		"default_task_queue": {
			"activities_per_second": 1.5
		},
		"slow_task_queue": {
			"activities_per_second": 2.5
		}
	}
	`
		var result raw.RevisionSetter

		err := json.Unmarshal([]byte(rawJSON), &result)
		assert.NoError(t, err)
	})
}

func TestPRRevision_Validate_Success(t *testing.T) {
	prRevision := &raw.RevisionSetter{
		URL: "https://test-url.com",
		BasicAuth: raw.BasicAuth{
			Username: "test-username",
			Password: "test-password",
		},
		DefaultTaskQueue: raw.TaskQueue{
			ActivitesPerSecond: 10.0,
		},
		SlowTaskQueue: raw.TaskQueue{
			ActivitesPerSecond: 1.5,
		},
	}
	assert.NoError(t, prRevision.Validate())
}

func TestPRRevision_Validate_Error(t *testing.T) {
	cases := []struct {
		description string
		subject     raw.RevisionSetter
	}{
		{
			description: "missing basic auth",
			subject: raw.RevisionSetter{
				URL: "https://tes-url.com",
				DefaultTaskQueue: raw.TaskQueue{
					ActivitesPerSecond: 1.5,
				},
				SlowTaskQueue: raw.TaskQueue{
					ActivitesPerSecond: 2.5,
				},
			},
		},
		{
			description: "missing password",
			subject: raw.RevisionSetter{
				URL: "https://tes-url.com",
				BasicAuth: raw.BasicAuth{
					Username: "test-username",
				},
				DefaultTaskQueue: raw.TaskQueue{
					ActivitesPerSecond: 1.5,
				},
				SlowTaskQueue: raw.TaskQueue{
					ActivitesPerSecond: 2.5,
				},
			},
		},
		{
			description: "missing username",
			subject: raw.RevisionSetter{
				URL: "https://tes-url.com",
				BasicAuth: raw.BasicAuth{
					Password: "test-password",
				},
				DefaultTaskQueue: raw.TaskQueue{
					ActivitesPerSecond: 1.5,
				},
				SlowTaskQueue: raw.TaskQueue{
					ActivitesPerSecond: 2.5,
				},
			},
		},
		{
			description: "missing url",
			subject: raw.RevisionSetter{
				BasicAuth: raw.BasicAuth{
					Username: "test-username",
					Password: "test-password",
				},
				DefaultTaskQueue: raw.TaskQueue{
					ActivitesPerSecond: 1.5,
				},
				SlowTaskQueue: raw.TaskQueue{
					ActivitesPerSecond: 2.5,
				},
			},
		},
		{
			description: "invalid url",
			subject: raw.RevisionSetter{
				URL: "tes-url&^*&.com",
				BasicAuth: raw.BasicAuth{
					Username: "test-username",
					Password: "test-password",
				},
				DefaultTaskQueue: raw.TaskQueue{
					ActivitesPerSecond: 1.5,
				},
				SlowTaskQueue: raw.TaskQueue{
					ActivitesPerSecond: 2.5,
				},
			},
		},
		{
			description: "missing activities_per_second_config",
			subject: raw.RevisionSetter{
				URL: "tes-url.com",
				BasicAuth: raw.BasicAuth{
					Username: "test-username",
					Password: "test-password",
				},
				DefaultTaskQueue: raw.TaskQueue{},
				SlowTaskQueue:    raw.TaskQueue{},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			assert.Error(t, c.subject.Validate())
		})
	}
}
