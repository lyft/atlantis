package deployment

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	internal_stow "github.com/runatlantis/atlantis/server/neptune/stow"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/root"
	"github.com/uber-go/tally/v4"
)

const OutputPrefix = "deployments"

func NewStore(stowClient *internal_stow.Client, scope tally.Scope) (*instrumentedStore, error) {
	return &instrumentedStore{
		store: &store{
			client: stowClient,
		},
		scope: scope.SubScope("store"),
	}, nil

}

type store struct {
	client *internal_stow.Client
}

func (s *store) GetDeploymentInfo(ctx context.Context, repoName string, rootName string) (*root.DeploymentInfo, error) {
	key := buildKey(repoName, rootName)

	reader, closer, err := s.client.Get(ctx, key)
	if err != nil {
		return nil, errors.Wrap(err, "getting item")
	}
	defer closer()

	decoder := json.NewDecoder(reader)

	var deploymentInfo root.DeploymentInfo
	err = decoder.Decode(&deploymentInfo)
	if err != nil {
		return nil, errors.Wrap(err, "decoding item")
	}

	return &deploymentInfo, nil
}

func (s *store) SetDeploymentInfo(ctx context.Context, deploymentInfo root.DeploymentInfo) error {
	key := buildKey(deploymentInfo.Repo.GetFullName(), deploymentInfo.Root.Name)
	object, err := json.Marshal(deploymentInfo)
	if err != nil {
		return errors.Wrap(err, "marshalling deployment info")
	}
	err = s.client.Set(ctx, key, object)
	if err != nil {
		return errors.Wrap(err, "writing to store")
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

func (i *instrumentedStore) GetDeploymentInfo(ctx context.Context, repoName string, rootName string) (*root.DeploymentInfo, error) {
	failureCount := i.scope.Counter("read_failure")
	successCount := i.scope.Counter("read_success")
	latency := i.scope.Timer("read_latency")
	span := latency.Start()
	defer span.Stop()

	deploymentInfo, err := i.store.GetDeploymentInfo(ctx, repoName, rootName)
	if err != nil {
		failureCount.Inc(1)
	}
	successCount.Inc(1)
	return deploymentInfo, err
}

func (i *instrumentedStore) SetDeploymentInfo(ctx context.Context, deploymentInfo root.DeploymentInfo) error {
	failureCount := i.scope.Counter("write_failure")
	successCount := i.scope.Counter("write_success")
	latency := i.scope.Timer("write_latency")
	span := latency.Start()
	defer span.Stop()

	err := i.store.SetDeploymentInfo(ctx, deploymentInfo)
	if err != nil {
		failureCount.Inc(1)
	}
	successCount.Inc(1)
	return err
}
