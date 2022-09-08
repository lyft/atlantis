package activities

import (
	"context"

	"github.com/runatlantis/atlantis/server/neptune/logger"
)

type dbActivities struct {
	Logger logger.Logger
}

type FetchLatestDeploymentRequest struct {
	RepositoryURL string
	RootName      string
}

type FetchLatestDeploymentResponse struct {
	ID       string
	Revision string
}

func (a *dbActivities) FetchLatestDeployment(ctx context.Context, request FetchLatestDeploymentRequest) (FetchLatestDeploymentResponse, error) {
	a.Logger.Info(ctx, "fetching latest deployment")

	return FetchLatestDeploymentResponse{
		ID:       "test-id",
		Revision: "1234",
	}, nil
}
