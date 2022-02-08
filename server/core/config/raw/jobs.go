package raw

import (
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/runatlantis/atlantis/server/core/config/valid"
)

/*
Ref: https://docs.aws.amazon.com/AmazonS3/latest/userguide/bucketnamingrules.html
S3 bucket naming rules:
1. Must be between 3 - 63 characters long
2. Must be lowercase letters, numbers, dots and hyphens.
*/
const ValidBucketNameRegEx = `^[a-z0-9-.]*$`

type Jobs struct {
	StorageBackend *StorageBackend `yaml:"storage-backend" json:"storage-backend"`
}

type StorageBackend struct {
	S3 *S3 `yaml:"s3" json:"s3"`
}

type S3 struct {
	BucketName string `yaml:"bucket-name" json:"bucket-name"`
}

func (j Jobs) Validate() error {
	return validation.ValidateStruct(&j,
		validation.Field(&j.StorageBackend),
	)
}

func (s StorageBackend) Validate() error {
	return validation.ValidateStruct(&s,
		validation.Field(&s.S3),
	)
}

func (s S3) Validate() error {
	return validation.ValidateStruct(&s,
		validation.Field(&s.BucketName, validation.Required),
	)
}

func (j *Jobs) ToValid() valid.Jobs {
	if j.StorageBackend != nil {
		return valid.Jobs{
			StorageBackend: &valid.StorageBackend{
				S3: &valid.S3{
					BucketName: j.StorageBackend.S3.BucketName,
				},
			},
		}
	}

	return valid.Jobs{}
}
