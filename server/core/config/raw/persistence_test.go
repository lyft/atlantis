package raw_test

import (
	"encoding/json"
	"testing"

	"github.com/runatlantis/atlantis/server/core/config/raw"
	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

func TestPersistence_Unmarshal(t *testing.T) {
	t.Run("yaml", func(t *testing.T) {

		rawYaml := `
default_store: default
prefix: test-prefix
data_stores:
  default:
    s3:
      bucket-name: atlantis-test
`

		var result raw.Persistence

		err := yaml.UnmarshalStrict([]byte(rawYaml), &result)
		assert.NoError(t, err)
	})

	t.Run("json", func(t *testing.T) {
		rawJSON := `
	{
		"default_store": "default",
		"prefix": "test-prefix",
		"data_stores": {
			"default": {
				"s3": {
					"bucket-name": "atlantis-test"
				}
			}
		}
	}
	`
		var result raw.Persistence

		err := json.Unmarshal([]byte(rawJSON), &result)
		assert.NoError(t, err)
	})

}

func TestPersistence_ValidateSuccess(t *testing.T) {
	cases := []struct {
		description string
		subject     raw.Persistence
	}{
		{
			description: "success",
			subject: raw.Persistence{
				DefaultStore: "default",
				Prefix:       "prefix",
				DataStores: map[string]raw.DataStore{
					"default": raw.DataStore{
						S3: &raw.S3{
							BucketName: "test-bucket",
						},
					},
				},
			},
		},
		{
			description: "success with job store override",
			subject: raw.Persistence{
				DefaultStore: "default",
				JobStore:     "job_store",
				DataStores: map[string]raw.DataStore{
					"default": raw.DataStore{
						S3: &raw.S3{
							BucketName: "test-bucket",
						},
					},
					"job_store": raw.DataStore{
						S3: &raw.S3{
							BucketName: "job-store-bucket",
						},
					},
				},
			},
		},
		{
			description: "success with deployment store override",
			subject: raw.Persistence{
				DefaultStore:    "default",
				DeploymentStore: "deployment_store",
				DataStores: map[string]raw.DataStore{
					"default": raw.DataStore{
						S3: &raw.S3{
							BucketName: "test-bucket",
						},
					},
					"deployment_store": raw.DataStore{
						S3: &raw.S3{
							BucketName: "deployment-store-bucket",
						},
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			assert.NoError(t, c.subject.Validate())
		})
	}

}

func TestPersistence_ValidateError(t *testing.T) {

	cases := []struct {
		description string
		subject     raw.Persistence
	}{
		{
			description: "default store not defined",
			subject: raw.Persistence{
				DefaultStore: "default",
				Prefix:       "prefix",
				DataStores: map[string]raw.DataStore{
					"another": raw.DataStore{
						S3: &raw.S3{
							BucketName: "test-bucket",
						},
					},
				},
			},
		},
		{
			description: "job store not defined",
			subject: raw.Persistence{
				DefaultStore: "default",
				JobStore:     "job_store",
				Prefix:       "prefix",
				DataStores: map[string]raw.DataStore{
					"default": raw.DataStore{
						S3: &raw.S3{
							BucketName: "test-bucket",
						},
					},
				},
			},
		},
		{
			description: "persistence store not defined",
			subject: raw.Persistence{
				DefaultStore:    "default",
				DeploymentStore: "deployment_store",
				Prefix:          "prefix",
				DataStores: map[string]raw.DataStore{
					"default": raw.DataStore{
						S3: &raw.S3{
							BucketName: "test-bucket",
						},
					},
				},
			},
		},
		{
			description: "bucket name not configured",
			subject: raw.Persistence{
				DefaultStore: "default",
				Prefix:       "prefix",
				DataStores: map[string]raw.DataStore{
					"default": raw.DataStore{
						S3: &raw.S3{},
					},
				},
			},
		},
		{
			description: "default store not configured",
			subject: raw.Persistence{
				Prefix: "prefix",
				DataStores: map[string]raw.DataStore{
					"default": raw.DataStore{
						S3: &raw.S3{},
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			assert.Error(t, c.subject.Validate())
		})
	}
}
