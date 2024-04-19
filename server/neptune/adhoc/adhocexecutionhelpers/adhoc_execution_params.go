package adhoc

import (
	"context"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/gateway/config"
	root_config "github.com/runatlantis/atlantis/server/neptune/gateway/config"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	internal_gh "github.com/runatlantis/atlantis/server/vcs/provider/github"
	"github.com/runatlantis/atlantis/server/vcs/provider/github/converter"
)

type AdhocTerraformWorkflowExecutionParams struct {
	Revision       string
	TerraformRoots []terraform.Root
	GithubRepo     github.Repo
	// Note that deploymentID is used in NewWorkflowStore(), but we don't care about that in adhoc mode so can leave it blank
}

func ConstructAdhocExecParams(
	ctx context.Context,
	repoName string,
	PRNum int,
	pullFetcher *internal_gh.PRFetcher,
	pullConverter converter.PullConverter,
	installationRetriever *internal_gh.InstallationRetriever,
	rootCfgBuilder *root_config.Builder) (AdhocTerraformWorkflowExecutionParams, error) {

	orgName := "lyft"
	installationToken, err := installationRetriever.FindOrganizationInstallation(ctx, orgName)
	if err != nil {
		return AdhocTerraformWorkflowExecutionParams{}, errors.Wrap(err, "finding organization installation")
	}

	// TODO: in the future, could potentially pass in the owner instead of hardcoding lyft
	ghCommit, err := pullFetcher.Fetch(ctx, installationToken.Token, orgName, repoName, PRNum)
	if err != nil {
		return AdhocTerraformWorkflowExecutionParams{}, errors.Wrap(err, "fetching commit")
	}

	actualCommit, err := pullConverter.Convert(ghCommit)
	if err != nil {
		return AdhocTerraformWorkflowExecutionParams{}, errors.Wrap(err, "converting commit")
	}

	opts := config.BuilderOptions{
		RepoFetcherOptions: &internal_gh.RepoFetcherOptions{
			CloneDepth: 1,
			SimplePath: true,
		},
	}

	rootCfgs, err := rootCfgBuilder.Build(ctx, &root_config.RepoCommit{
		Repo:          actualCommit.HeadRepo,
		Branch:        actualCommit.HeadBranch,
		Sha:           actualCommit.HeadCommit,
		OptionalPRNum: actualCommit.Num,
	}, installationToken.Token, opts)
	if err != nil {
		return AdhocTerraformWorkflowExecutionParams{}, errors.Wrap(err, "building root cfgs")
	}

	rootCfgBuilder.Logger.Info("getting roots from merged project cfgs")
	roots := getRootsFromMergedProjectCfgs(rootCfgs)

	rootCfgBuilder.Logger.Info("returning adhocexecution params")
	return AdhocTerraformWorkflowExecutionParams{
		Revision: actualCommit.HeadCommit,
		GithubRepo: github.Repo{
			Owner:         orgName,
			Name:          repoName,
			URL:           actualCommit.HeadRepo.CloneURL,
			DefaultBranch: actualCommit.HeadRepo.DefaultBranch,
			Credentials:   github.AppCredentials{InstallationToken: installationToken.Token},
		},
		TerraformRoots: roots,
	}, nil
}
