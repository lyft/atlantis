package adhoc

import (
	"context"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/adhoc/adhocgithubhelpers"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
)

type AdhocTerraformWorkflowExecutionParams struct {
	Revision       string
	TerraformRoots []terraform.Root
	GithubRepo     github.Repo
	// Note that deploymentID is used in NewWorkflowStore(), but we don't care about that in adhoc mode so can leave it blank
}

func ConstructAdhocExecParamsWithRootCfgBuilderAndRepoRetriever(ctx context.Context, repoName string, revision string, githubRetriever *adhocgithubhelpers.AdhocGithubRetriever) (AdhocTerraformWorkflowExecutionParams, error) {
	// TODO: in the future, could potentially pass in the owner instead of hardcoding lyft
	repo, err := githubRetriever.GetRepository(ctx, "lyft", repoName)
	if err != nil {
		return AdhocTerraformWorkflowExecutionParams{}, errors.Wrap(err, "getting repo")
	}

	return AdhocTerraformWorkflowExecutionParams{
		Revision:   revision,
		GithubRepo: repo,
	}, nil
}
