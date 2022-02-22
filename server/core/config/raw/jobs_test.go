package raw_test

import (
	"encoding/json"
	"testing"

	"github.com/runatlantis/atlantis/server/core/config/raw"
	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

func TestJobs_Unmarshal(t *testing.T) {
	t.Run("auth-type: iam yaml", func(t *testing.T) {

		rawYaml := `
storage-backend:
  s3:
    bucket-name: atlantis-test
    auth-type: iam
`

		var result raw.Jobs

		err := yaml.UnmarshalStrict([]byte(rawYaml), &result)
		assert.NoError(t, err)
	})

	t.Run("auth-type: iam json", func(t *testing.T) {
		rawJSON := `
	{
		"storage-backend": {
			"s3": {
				"bucket-name": "atlantis-test",
				"auth-type": "iam"
			}
		}
	}
	`

		var result raw.Jobs

		err := json.Unmarshal([]byte(rawJSON), &result)
		assert.NoError(t, err)
	})

	t.Run("auth-type: accesskeys yaml", func(t *testing.T) {

		rawYaml := `
storage-backend:
  s3:
    bucket-name: atlantis-test
    auth-type: accesskey
    access-key: 
        access-id: test-id
        secret-key: test-secret-key
`

		var result raw.Jobs

		err := yaml.UnmarshalStrict([]byte(rawYaml), &result)
		assert.NoError(t, err)
	})

	t.Run("auth-type: accesskeys json", func(t *testing.T) {
		rawJSON := `
	{
		"storage-backend": {
			"s3": {
				"bucket-name": "atlantis-test",
				"auth-type": "accesskey",
				"access-key": {
					"access-id": "test-id",
					"secret-key": "test-secret-key"
				}
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
			description: "iam",
			subject: raw.Jobs{
				StorageBackend: &raw.StorageBackend{
					S3: &raw.S3{
						BucketName: "test-bucket",
						AuthType:   "iam",
					},
				},
			},
		},
		{
			description: "accesskey",
			subject: raw.Jobs{
				StorageBackend: &raw.StorageBackend{
					S3: &raw.S3{
						BucketName: "test-bucket",
						AuthType:   "accesskey",
						AccessKey: &raw.AccessKey{
							ConfigAccessKeyID: "test-id",
							ConfigSecretKey:   "test-secret",
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

func TestJobs_ValidateError(t *testing.T) {
	cases := []struct {
		description string
		subject     raw.Jobs
	}{
		{
			description: "bucket name not specified",
			subject: raw.Jobs{
				StorageBackend: &raw.StorageBackend{
					S3: &raw.S3{
						AuthType: "iam",
					},
				},
			},
		},
		{
			description: "auth type not specified",
			subject: raw.Jobs{
				StorageBackend: &raw.StorageBackend{
					S3: &raw.S3{
						BucketName: "test-bucket",
					},
				},
			},
		},
		{
			description: "access keys not specified when auth-type set to accesskey",
			subject: raw.Jobs{
				StorageBackend: &raw.StorageBackend{
					S3: &raw.S3{
						AuthType:   "accesskey",
						BucketName: "test-bucket",
					},
				},
			},
		},
		{
			description: "access key ID not specified when auth-type set to accesskey",
			subject: raw.Jobs{
				StorageBackend: &raw.StorageBackend{
					S3: &raw.S3{
						AuthType:   "accesskey",
						BucketName: "test-bucket",
						AccessKey: &raw.AccessKey{
							ConfigSecretKey: "test-secret",
						},
					},
				},
			},
		},
		{
			description: "secret key not specified when auth-type set to accesskey",
			subject: raw.Jobs{
				StorageBackend: &raw.StorageBackend{
					S3: &raw.S3{
						AuthType:   "accesskey",
						BucketName: "test-bucket",
						AccessKey: &raw.AccessKey{
							ConfigAccessKeyID: "test-id",
						},
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
