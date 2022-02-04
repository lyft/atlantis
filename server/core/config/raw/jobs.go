package raw

import (
	"errors"
	"regexp"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/runatlantis/atlantis/server/core/config/valid"
)

const ValidBucketNameRegEx = "^[a-z0-9_.]*$"

type Jobs struct {
	StorageBackend *StorageBackend `yaml:"storage-backend" json:"storage-backend"`
}

type StorageBackend struct {
	S3 *S3 `yaml:"s3" json:"s3"`

	// Add new storage backends such as local storage
}

type S3 struct {
	BucketName string `yaml:"bucket-name" json:"bucket-name"`
}

func (j *Jobs) Validate() error {
	return validation.ValidateStruct(&j,
		validation.Field(&j.StorageBackend),
	)
}

func (s *StorageBackend) Validate() error {
	// Atleast one of the storage backends need to be configured
	// Error out if more than one configured.
	return validation.ValidateStruct(&s,
		validation.Field(&s.S3),
	)
}

func (s *S3) Validate() error {
	return validation.ValidateStruct(s,
		validation.Field(&s.BucketName, validation.Required, validation.Length(3, 63)),
		validation.Field(&s.BucketName, validation.By(s.validateBucketName)),
	)
}

func (s *S3) validateBucketName(value interface{}) error {
	bucketName, _ := value.(string)
	if len(bucketName) < 3 || len(bucketName) > 63 {
		return errors.New("s3 bucket names must be between 3 and 63 characters")
	}

	re := regexp.MustCompile(ValidBucketNameRegEx)
	if !re.MatchString(bucketName) {
		return errors.New("s3 bucket names can only consist of lowercase letters, numbers, dots(.) and hyphens(-)")
	}
	return nil
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
