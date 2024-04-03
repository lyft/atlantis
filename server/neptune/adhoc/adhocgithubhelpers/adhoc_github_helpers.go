package adhocgithubhelpers

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/models"
	adhoc "github.com/runatlantis/atlantis/server/neptune/adhoc/adhocexecutionhelpers"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	internal "github.com/runatlantis/atlantis/server/vcs/provider/github"
)

type repoRetriever interface {
	Get(ctx context.Context, installationToken int64, owner, repo string) (models.Repo, error)
}

type installationRetriever interface {
	FindOrganizationInstallation(ctx context.Context, org string) (internal.Installation, error)
}

type AdhocGithubRetriever struct {
	RepoRetriever         repoRetriever
	InstallationRetriever installationRetriever
}

func ConstructAdhocExecParamsWithRootCfgBuilderAndRepoRetriever(ctx context.Context, repoName string, revision string, githubRetriever *AdhocGithubRetriever) (adhoc.AdhocTerraformWorkflowExecutionParams, error) {
	adhocExecParams := adhoc.AdhocTerraformWorkflowExecutionParams{}
	// TODO: in the future, could potentially pass in the owner instead of hardcoding lyft
	repo, token, err := githubRetriever.GetRepositoryAndToken(ctx, "lyft", repoName)
	if err != nil {
		return adhocExecParams, errors.Wrap(err, "getting repo")
	}

	githubRepo := convertRepoToGithubRepo(repo, token)

	adhocExecParams.GithubRepo = githubRepo
	adhocExecParams.Revision = revision

	return adhocExecParams, nil
}

func (r *AdhocGithubRetriever) GetRepositoryAndToken(ctx context.Context, owner string, repoName string) (models.Repo, int64, error) {
	installation, err := r.InstallationRetriever.FindOrganizationInstallation(ctx, owner)
	if err != nil {
		return models.Repo{}, installation.Token, errors.Wrap(err, "finding installation")
	}

	repo, err := r.RepoRetriever.Get(ctx, installation.Token, owner, repoName)
	if err != nil {
		return repo, installation.Token, errors.Wrap(err, "getting repo")
	}

	if len(repo.DefaultBranch) == 0 {
		return repo, installation.Token, fmt.Errorf("default branch was nil, this is a bug on github's side")
	}

	return repo, installation.Token, nil
}

func convertRepoToGithubRepo(repo models.Repo, token int64) github.Repo {
	return github.Repo{
		Owner:         repo.Owner,
		Name:          repo.Name,
		URL:           repo.CloneURL,
		DefaultBranch: repo.DefaultBranch,
		Credentials: github.AppCredentials{
			InstallationToken: token,
		},
	}
}
