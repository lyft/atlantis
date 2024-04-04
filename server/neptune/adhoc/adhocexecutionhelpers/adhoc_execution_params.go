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

func (a *AdhocTerraformWorkflowExecutionParams) ConstructAdhocExecParamsWithRootCfgBuilderAndRepoRetriever(ctx context.Context, repoName string, revision string, githubRetriever *adhocgithubhelpers.AdhocGithubRetriever) error {
	// TODO: in the future, could potentially pass in the owner instead of hardcoding lyft
	repo, token, err := githubRetriever.GetRepositoryAndToken(ctx, "lyft", repoName)
	if err != nil {
		return errors.Wrap(err, "getting repo")
	}

	githubRepo := adhocgithubhelpers.ConvertRepoToGithubRepo(repo, token)

	a.GithubRepo = githubRepo
	a.Revision = revision

	return nil
}
