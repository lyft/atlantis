package raw_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/runatlantis/atlantis/server/core/config/raw"
	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

func TestJobs_Unmarshal(t *testing.T) {
	t.Run("yaml", func(t *testing.T) {

		rawYaml := `
storage-backend:
  s3:
    bucket-name: atlantis-test
`

		var result raw.Jobs

		err := yaml.UnmarshalStrict([]byte(rawYaml), &result)
		assert.NoError(t, err)
	})

	t.Run("json", func(t *testing.T) {
		rawJSON := `
	{
		"storage-backend": {
			"s3": {
				"bucket-name": "atlantis-test"
			}
		}
	}
	`

		var result raw.Jobs

		err := json.Unmarshal([]byte(rawJSON), &result)
		assert.NoError(t, err)
	})
}

func TestJobs_Validate_Success(t *testing.T) {
	cases := []struct {
		description string
		subject     raw.Jobs
	}{
		{
			description: "success",
			subject: raw.Jobs{
				StorageBackend: &raw.StorageBackend{
					S3: &raw.S3{
						BucketName: "test-.bucket",
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

func TestJobs_ValidateError(t *testing.T) {
	cases := []struct {
		description string
		subject     raw.Jobs
	}{
		{
			description: "length lt 3",
			subject: raw.Jobs{
				StorageBackend: &raw.StorageBackend{
					S3: &raw.S3{
						BucketName: "aa",
					},
				},
			},
		},
		{
			description: "lengtth gt 63",
			subject: raw.Jobs{
				StorageBackend: &raw.StorageBackend{
					S3: &raw.S3{
						BucketName: strings.Repeat("a", 65),
					},
				},
			},
		},
		{
			description: "invalid chars",
			subject: raw.Jobs{
				StorageBackend: &raw.StorageBackend{
					S3: &raw.S3{
						BucketName: "*&hello",
					},
				},
			},
		},
		{
			description: "starts with a non-letter and non-number",
			subject: raw.Jobs{
				StorageBackend: &raw.StorageBackend{
					S3: &raw.S3{
						BucketName: "-bucket-name",
					},
				},
			},
		},
		{
			description: "ends with a non-letter and non-number",
			subject: raw.Jobs{
				StorageBackend: &raw.StorageBackend{
					S3: &raw.S3{
						BucketName: "bucket-name-",
					},
				},
			},
		},
		{
			description: "uppercase letters",
			subject: raw.Jobs{
				StorageBackend: &raw.StorageBackend{
					S3: &raw.S3{
						BucketName: "Bucket-name",
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
