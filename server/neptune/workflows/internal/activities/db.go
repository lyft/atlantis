package activities

import (
	"context"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/logger"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities/deployment"
)

// Downloader is implemented by manager.Downloader
type store interface {
	GetDeploymentInfo(ctx context.Context, repo string, root string) (*deployment.DeploymentInfo, error)
	SetDeploymentInfo(ctx context.Context, repo string, root string, ddeploymentInfo deployment.DeploymentInfo) error
}

type dbActivities struct {
	DeploymentInfoStore store
	BucketName          string
}

type FetchLatestDeploymentRequest struct {
	RepositoryName string
	RootName       string
}

type FetchLatestDeploymentResponse struct {
	DeploymentInfo deployment.DeploymentInfo
}

func (a *dbActivities) FetchLatestDeployment(ctx context.Context, request FetchLatestDeploymentRequest) (FetchLatestDeploymentResponse, error) {
	logger.Info(ctx, "fetching latest deployment")
	deploymentInfo, err := a.DeploymentInfoStore.GetDeploymentInfo(ctx, request.RepositoryName, request.RootName)
	if err != nil {
		return FetchLatestDeploymentResponse{}, errors.Wrapf(err, "fetching deployment info for %s/%s", request.RepositoryName, request.RootName)
	}
	return FetchLatestDeploymentResponse{
		DeploymentInfo: *deploymentInfo,
	}, nil
}

type StoreLatestDeploymentRequest struct {
	DeploymentInfo deployment.DeploymentInfo
	RepoName       string
}

func (a *dbActivities) StoreLatestDeployment(ctx context.Context, request StoreLatestDeploymentRequest) error {
	logger.Info(ctx, "storing latest deployment")
	err := a.DeploymentInfoStore.SetDeploymentInfo(ctx, request.RepoName, request.DeploymentInfo.Root.Name, request.DeploymentInfo)
	if err != nil {
		return errors.Wrapf(err, "uploading deployment info for %s", request.DeploymentInfo.ID)
	}

	return nil
}
