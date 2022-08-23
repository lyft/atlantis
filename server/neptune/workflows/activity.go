package workflows

import (
	"github.com/hashicorp/go-version"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/runtime/cache"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/uber-go/tally/v4"
)

type Activities struct {
	activities.Deploy
	activities.Terraform
}

func NewActivities(
	appConfig githubapp.Config,
	scope tally.Scope,
	versionCache cache.ExecutionVersionCache,
	defaultTfVersion *version.Version,
	terraformBinDir string,
) (*Activities, error) {
	deployActivities, err := activities.NewDeploy(appConfig, scope)
	if err != nil {
		return nil, errors.Wrap(err, "initializing deploy activities")
	}

	terraformActivites := activities.NewTerraform(versionCache, defaultTfVersion, terraformBinDir)
	return &Activities{
		Deploy:    *deployActivities,
		Terraform: *terraformActivites,
	}, nil
}
