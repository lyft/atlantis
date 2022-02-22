package raw

import (
	"fmt"
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

func (j *Jobs) ToValid() valid.Jobs {
	if j.StorageBackend == nil {
		return valid.Jobs{}
	}

	return valid.Jobs{
		StorageBackend: j.StorageBackend.ToValid(),
	}
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

// Here we know that only one storage backend is configured
// Find the non-nil one and return the valid config
func (s *StorageBackend) ToValid() *valid.StorageBackend {
	switch {
	case s.S3 != nil:
		return &valid.StorageBackend{
			S3: s.S3.ToValid(),
		}
	default:
		return &valid.StorageBackend{}
	}
}

type S3 struct {
	BucketName string     `yaml:"bucket-name" json:"bucket-name"`
	AuthType   string     `yaml:"auth-type" json:"auth-type"`
	AccessKey  *AccessKey `yaml:"access-key" json:"access-key"`
}

// TODO: Use validation.When() to do conditional validation based on Auth Type
func (s S3) Validate() error {
	if s.AuthType == "" {
		return fmt.Errorf("AuthType must be configured: Valid auth types are: %s and %s", IamAuthType, AccessKeyAuthType)
	} else if s.AuthType == AccessKeyAuthType {
		return validation.ValidateStruct(&s,
			validation.Field(&s.BucketName, validation.Required),
			validation.Field(&s.AuthType, validation.Required),
			validation.Field(&s.AccessKey, validation.Required),
		)
	} else if s.AuthType == IamAuthType {
		return validation.ValidateStruct(&s,
			validation.Field(&s.BucketName, validation.Required),
			validation.Field(&s.AuthType, validation.Required),
		)
	} else {
		return fmt.Errorf("invalid auth type. Valid auth types are: %s and %s", IamAuthType, AccessKeyAuthType)
	}
}

func (s S3) ToValid() *valid.S3 {

	switch s.AuthType {
	case IamAuthType:
		return &valid.S3{
			AuthType:   valid.Iam,
			BucketName: s.resolveBucketName(),
		}
	case AccessKeyAuthType:
		return &valid.S3{
			AuthType:    valid.AccessKey,
			BucketName:  s.resolveBucketName(),
			AccessKeyID: s.AccessKey.ConfigAccessKeyID,
			SecretKey:   s.AccessKey.ConfigSecretKey,
		}
	default:
		return nil
	}
}

func (s *S3) resolveBucketName() string {
	// env variable
	if s.BucketName[0:1] == "$" {
		return os.Getenv(s.BucketName[1:])
	}
	return s.BucketName
}

type AccessKey struct {
	ConfigAccessKeyID string `yaml:"access-id" json:"access-id"`
	ConfigSecretKey   string `yaml:"secret-key" json:"secret-key"`
}

func (a AccessKey) Validate() error {
	return validation.ValidateStruct(&a,
		validation.Field(&a.ConfigAccessKeyID, validation.Required),
		validation.Field(&a.ConfigSecretKey, validation.Required),
	)
}
