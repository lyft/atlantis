package deployment

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/stow"
	internal_stow "github.com/runatlantis/atlantis/server/neptune/stow"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/root"
)

const OutputPrefix = "deployments"

type client interface {
	Get(ctx context.Context, key string) (io.ReadCloser, stow.CloserFn, error)
	Set(ctx context.Context, key string, object []byte) error
}

func NewStore(stowClient client) (*store, error) {
	return &store{
		stowClient: stowClient,
	}, nil

}

type store struct {
	stowClient client
}

func (s *store) GetDeploymentInfo(ctx context.Context, repoName string, rootName string) (*root.DeploymentInfo, error) {
	key := BuildKey(repoName, rootName)

	reader, closer, err := s.stowClient.Get(ctx, key)
	if err != nil {
		switch err.(type) {

		// Fail if container is not found
		case *internal_stow.ContainerNotFoundError:
			return nil, err

		// First deploy for this root
		case *internal_stow.ItemNotFoundError:
			return &root.DeploymentInfo{}, nil

		default:
			return nil, errors.Wrap(err, "getting item")
		}
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
	key := BuildKey(deploymentInfo.Repo.GetFullName(), deploymentInfo.Root.Name)
	object, err := json.Marshal(deploymentInfo)
	if err != nil {
		return errors.Wrap(err, "marshalling deployment info")
	}

	err = s.stowClient.Set(ctx, key, object)
	if err != nil {
		return errors.Wrap(err, "writing to store")
	}
	return nil
}

func BuildKey(repo string, root string) string {
	return fmt.Sprintf("%s/%s/%s/deployment.json", OutputPrefix, repo, root)
}