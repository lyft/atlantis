package raw

import (
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/runatlantis/atlantis/server/core/config/valid"
)

type Deployments struct {
	StorageBackend *StorageBackend `yaml:"storage-backend" json:"storage-backend"`
}

func (d Deployments) Validate() error {
	return validation.ValidateStruct(&d,
		validation.Field(&d.StorageBackend),
	)
}

func (d *Deployments) ToValid() valid.Deployments {
	if d.StorageBackend == nil {
		return valid.Deployments{}
	}

	storageBackend := d.StorageBackend.ToValid()
	return valid.Deployments{
		StorageBackend: &storageBackend,
	}
}
