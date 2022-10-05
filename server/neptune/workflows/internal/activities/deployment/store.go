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

func NewStore(deploymentCfg valid.Deployments, scope tally.Scope) (*instrumentedStore, error) {
	if deploymentCfg.StorageBackend == nil {
		deploymentCfg = valid.Deployments{
			StorageBackend: &valid.StorageBackend{
				BackendConfig: &valid.S3{
					BucketName: "atlantis-staging-jobs",
				},
			},
		}
		// return nil, errors.New("error initializing deployment info store")
	}

	config := deploymentCfg.StorageBackend.BackendConfig.GetConfigMap()
	backend := deploymentCfg.StorageBackend.BackendConfig.GetConfiguredBackend()
	containerName := deploymentCfg.StorageBackend.BackendConfig.GetContainerName()

	location, err := stow.Dial(backend, config)
	if err != nil {
		return nil, err
	}

	return &instrumentedStore{
		store: &store{
			location:      location,
			containerName: containerName,
		},
		scope: scope.SubScope("store"),
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

	key := buildKey(repo, root)
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

	key := buildKey(repo, root)
	_, err = container.Put(key, bytes.NewReader(object), int64(len(object)), nil)
	if err != nil {
		return errors.Wrap(err, "writing to container")
	}

	return nil
}

func buildKey(repo string, root string) string {
	return fmt.Sprintf("%s/%s/%s/deployment.json", OutputPrefix, repo, root)
}

type instrumentedStore struct {
	*store
	scope tally.Scope
}

func (i *instrumentedStore) GetDeploymentInfo(ctx context.Context, repo string, root string) (*DeploymentInfo, error) {
	failureCount := i.scope.Counter("read_failure")
	successCount := i.scope.Counter("read_success")
	latency := i.scope.Timer("read_latency")
	span := latency.Start()
	defer span.Stop()

	deploymentInfo, err := i.store.GetDeploymentInfo(ctx, repo, root)
	if err != nil {
		failureCount.Inc(1)
	}
	successCount.Inc(1)
	return deploymentInfo, err
}

func (i *instrumentedStore) SetDeploymentInfo(ctx context.Context, repo string, root string, deploymentInfo DeploymentInfo) error {
	failureCount := i.scope.Counter("write_failure")
	successCount := i.scope.Counter("write_success")
	latency := i.scope.Timer("write_latency")
	span := latency.Start()
	defer span.Stop()

	err := i.store.SetDeploymentInfo(ctx, repo, root, deploymentInfo)
	if err != nil {
		failureCount.Inc(1)
	}
	successCount.Inc(1)
	return err
}
