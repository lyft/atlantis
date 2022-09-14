package terraform

import (
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/root"
	"go.temporal.io/sdk/workflow"
)

type RootFetcher struct {
	Request Request
	Ga      githubActivities
	Ta      terraformActivities
}

// Fetch returns a local root and a cleanup function
func (r *RootFetcher) Fetch(ctx workflow.Context) (*root.LocalRoot, func() error, error) {
	var fetchRootResponse activities.FetchRootResponse
	err := workflow.ExecuteActivity(ctx, r.Ga.FetchRoot, activities.FetchRootRequest{
		Repo:         r.Request.Repo,
		Root:         r.Request.Root,
		DeploymentId: r.Request.DeploymentId,
		Revision:     r.Request.Revision,
	}).Get(ctx, &fetchRootResponse)

	if err != nil {
		return nil, nil, err
	}

	return fetchRootResponse.LocalRoot, func() error {
		var cleanupResponse activities.CleanupResponse
		err = workflow.ExecuteActivity(ctx, r.Ta.Cleanup, activities.CleanupRequest{
			LocalRoot: fetchRootResponse.LocalRoot,
		}).Get(ctx, &cleanupResponse)
		if err != nil {
			return errors.Wrap(err, "cleaning up")
		}
		return nil
	}, nil
}