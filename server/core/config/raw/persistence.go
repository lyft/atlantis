package raw

import (
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/graymeta/stow"
	stow_s3 "github.com/graymeta/stow/s3"
	"github.com/runatlantis/atlantis/server/core/config/valid"
)

type Persistence struct {
	// DefaultStore is the name of the default data store to use
	DefaultStore string `yaml:"default_store" json:"default_store"`
	// DataStores contains the configuration for all datastores
	DataStores DataStores `yaml:"data_stores" json:"data_stores"`
	// Adds a prefix to the storage container
	Prefix string `yaml:"prefix" json:"prefix"`

	DeploymentStore string `yaml:"deployment_store" json:"deployment_store"`
	JobStore        string `yaml:"job_store" json:"job_store"`
}

func (p Persistence) Validate() error {
	// Get all configured data stores
	dsNames := []interface{}{}
	for dsName := range p.DataStores {
		dsNames = append(dsNames, dsName)
	}

	return validation.ValidateStruct(&p,
		validation.Field(&p.DefaultStore, validation.In(dsNames...)),
		validation.Field(&p.DeploymentStore, validation.In(dsNames...)),
		validation.Field(&p.JobStore, validation.In(dsNames...)),
		validation.Field(&p.DataStores),
	)
}

func (p Persistence) ToValid() valid.Persistence {
	validDefaultStore := buildValidStore(p.DataStores[p.DefaultStore])

	// Override if configured
	validJobStore := validDefaultStore
	if p.JobStore != "" {
		validJobStore = buildValidStore(p.DataStores[p.JobStore])
	} else {
		validJobStore = validDefaultStore
	}

	// Override if configured
	validDeploymentStore := validDefaultStore
	if p.DeploymentStore != "" {
		validDeploymentStore = buildValidStore(p.DataStores[p.DeploymentStore])
	}

	return valid.Persistence{
		Deployments: validDeploymentStore,
		Jobs:        validJobStore,
	}
}

func buildValidStore(dataStore DataStore) valid.StoreConfig {
	var validStore valid.StoreConfig

	// Serially checks for non-nil supported backends
	switch {
	case dataStore.S3 != nil:
		validStore = valid.StoreConfig{
			ContainerName: dataStore.S3.BucketName,
			BackendType:   valid.S3Backend,

			// Hard coding iam auth type since we only support this for now
			Config: stow.ConfigMap{
				stow_s3.ConfigAuthType: "iam",
			},
		}
	}
	return validStore
}

type DataStores map[string]DataStore

type DataStore struct {
	S3 *S3 `yaml:"s3" json:"s3"`

	// Add other supported data stores in the future
}

func (ds DataStore) Validate() error {
	return validation.ValidateStruct(&ds, validation.Field(&ds.S3))
}
