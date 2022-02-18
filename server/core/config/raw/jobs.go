package raw

import (
	"os"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/runatlantis/atlantis/server/core/config/valid"
)

const (
	IamAuthType       = "iam"
	AccessKeyAuthType = "accesskey"
)

type Jobs struct {
	StorageBackend *StorageBackend `yaml:"storage-backend" json:"storage-backend"`
}

func (j Jobs) Validate() error {
	return validation.ValidateStruct(&j,
		validation.Field(&j.StorageBackend),
	)
}

type StorageBackend struct {
	S3 *S3 `yaml:"s3" json:"s3"`
}

// Validate only one storage backend is configured when adding additonal storage backends
// using ozzo conditional validation
func (s StorageBackend) Validate() error {
	return validation.ValidateStruct(&s,
		validation.Field(&s.S3),
	)
}

type S3 struct {
	BucketName string     `yaml:"bucket-name" json:"bucket-name"`
	AuthType   string     `yaml:"auth-type" json:"auth-type"`
	AccessKey  *AccessKey `yaml:"access-key" json:"access-key"`
}

type AccessKey struct {
	ConfigAccessKeyID string `yaml:"access-id" json:"access-id"`
	ConfigSecretKey   string `yaml:"secret-key" json:"secret-key"`
}

// TODO: Add validation to check AccessKeys are configured when AuthType set to accesskeys
func (s S3) Validate() error {
	return validation.ValidateStruct(&s,
		validation.Field(&s.BucketName, validation.Required),
		validation.Field(&s.AuthType, validation.Required, validation.In(IamAuthType, AccessKeyAuthType)),
	)
}

func (s S3) getValidAuthType() valid.AuthType {
	if s.AuthType == IamAuthType {
		return valid.Iam
	} else if s.AuthType == AccessKeyAuthType {
		return valid.AccessKey
	}
	return 0
}

func (j *Jobs) ToValid() valid.Jobs {
	if j.StorageBackend == nil {
		return valid.Jobs{}
	}

	// Here we have already validated that only one storage backend is configured
	// Switch through all the storage backends and return the non-nil one
	s := j.StorageBackend
	switch {
	case s.S3 != nil:
		return valid.Jobs{
			StorageBackend: &valid.StorageBackend{
				S3: &valid.S3{
					BucketName: s.S3.resolveBucketName(),
					AuthType:   s.S3.getValidAuthType(),
				},
			},
		}
	default:
		return valid.Jobs{}
	}
}

func (s *S3) resolveBucketName() string {
	if s.BucketName[0:1] == "$" {
		return os.Getenv(s.BucketName[1:])
	}
	return s.BucketName
}
