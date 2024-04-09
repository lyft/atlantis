package adhoc

import (
	"context"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/adhoc/adhocgithubhelpers"
	"github.com/runatlantis/atlantis/server/neptune/gateway/config"
	root_config "github.com/runatlantis/atlantis/server/neptune/gateway/config"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	internal_gh "github.com/runatlantis/atlantis/server/vcs/provider/github"
)

type AdhocTerraformWorkflowExecutionParams struct {
	Revision       string
	TerraformRoots []terraform.Root
	GithubRepo     github.Repo
	// Note that deploymentID is used in NewWorkflowStore(), but we don't care about that in adhoc mode so can leave it blank
}

func ConstructAdhocExecParamsWithRootCfgBuilderAndRepoRetriever(ctx context.Context, repoName string, revision string, githubRetriever *adhocgithubhelpers.AdhocGithubRetriever, rootCfgBuilder *root_config.Builder) (AdhocTerraformWorkflowExecutionParams, error) {
	// TODO: in the future, could potentially pass in the owner instead of hardcoding lyft
	repo, err := githubRetriever.GetRepository(ctx, "lyft", repoName)
	if err != nil {
		return AdhocTerraformWorkflowExecutionParams{}, errors.Wrap(err, "getting repo")
	}

	opts := config.BuilderOptions{
		RepoFetcherOptions: &internal_gh.RepoFetcherOptions{
			CloneDepth: 1,
		},
	}

	rootCfgs, err := rootCfgBuilder.Build(ctx, &root_config.RepoCommit{}, repo.Credentials.InstallationToken, opts)
	if err != nil {
		return AdhocTerraformWorkflowExecutionParams{}, errors.Wrap(err, "building root cfgs")
	}

	roots, err := getRootsFromMergedProjectCfgs(rootCfgs)
	if err != nil {
		return AdhocTerraformWorkflowExecutionParams{}, errors.Wrap(err, "getting roots from merged project cfgs")
	}

	return AdhocTerraformWorkflowExecutionParams{
		Revision:       revision,
		GithubRepo:     repo,
		TerraformRoots: roots,
	}, nil
}