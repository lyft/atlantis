package deployment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/graymeta/stow"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/root"
	"github.com/uber-go/tally/v4"
)

const OutputPrefix = "deployments"

type DeploymentInfo struct {
	ID         string
	CheckRunID int64
	Revision   string
	Root       root.Root
	RepoName   string
}

type Store interface {
	GetDeploymentInfo(repo string, root string) (*DeploymentInfo, error)
	SetDeploymentInfo(repo string, root string, deploymentInfo DeploymentInfo) error
}

func NewStore(deployments valid.Deployments, scope tally.Scope) (*store, error) {
	if deployments.StorageBackend == nil {
		deployments = valid.Deployments{
			StorageBackend: &valid.StorageBackend{
				BackendConfig: &valid.S3{
					BucketName: "atlantis-staging-jobs",
				},
			},
		}
		// return nil, errors.New("error initializing deployment info store")
	}

	config := deployments.StorageBackend.BackendConfig.GetConfigMap()
	backend := deployments.StorageBackend.BackendConfig.GetConfiguredBackend()
	containerName := deployments.StorageBackend.BackendConfig.GetContainerName()

	location, err := stow.Dial(backend, config)
	if err != nil {
		return nil, err
	}

	return &store{
		location:      location,
		containerName: containerName,
	}, nil

}

type store struct {
	location      stow.Location
	containerName string
}

func (s *store) GetDeploymentInfo(ctx context.Context, repo string, root string) (*DeploymentInfo, error) {
	container, err := s.location.Container(s.containerName)
	if err != nil {
		return nil, errors.Wrap(err, "resolving container")
	}

	key := fmt.Sprintf("%s/%s/%s/deployment.json", OutputPrefix, repo, root)
	item, err := container.Item(key)
	if err != nil {
		return nil, errors.Wrap(err, "getting item")
	}

	r, err := item.Open()
	if err != nil {
		return nil, errors.Wrap(err, "reading item")
	}

	decoder := json.NewDecoder(r)

	var deploymentInfo DeploymentInfo
	err = decoder.Decode(&deploymentInfo)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshalling item")
	}

	return &deploymentInfo, nil
}

func (s *store) SetDeploymentInfo(ctx context.Context, repo string, root string, deploymentInfo DeploymentInfo) error {
	container, err := s.location.Container(s.containerName)
	if err != nil {
		return errors.Wrap(err, "resolving container")
	}

	object, err := json.Marshal(deploymentInfo)
	if err != nil {
		return errors.Wrap(err, "marshalling deployment info")
	}

	key := fmt.Sprintf("%s/%s/%s/deployment.json", OutputPrefix, repo, root)
	_, err = container.Put(key, bytes.NewReader(object), int64(len(object)), nil)
	if err != nil {
		return errors.Wrap(err, "writing to container")
	}

	return nil
}
