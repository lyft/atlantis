package activities

import (
	"context"

	"github.com/runatlantis/atlantis/server/neptune/logger"
)

type storeActivities struct {
}

type FetchLatestDeploymentRequest struct {
	RepositoryName string
	RootName       string
}

type FetchLatestDeploymentResponse struct {
	ID       string
	Revision string
}

func (a *storeActivities) FetchLatestDeployment(ctx context.Context, request FetchLatestDeploymentRequest) (FetchLatestDeploymentResponse, error) {
	logger.Info(ctx, "fetching latest deployment")

	return FetchLatestDeploymentResponse{
		ID:       "test-id",
		Revision: "1234",
	}, nil
}
