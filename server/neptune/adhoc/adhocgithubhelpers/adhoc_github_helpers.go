package adhocgithubhelpers

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/models"
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

func (r *AdhocGithubRetriever) GetRepository(ctx context.Context, owner string, repoName string) (github.Repo, error) {
	installation, err := r.InstallationRetriever.FindOrganizationInstallation(ctx, owner)
	if err != nil {
		return github.Repo{}, errors.Wrap(err, "finding installation")
	}

	repo, err := r.RepoRetriever.Get(ctx, installation.Token, owner, repoName)
	if err != nil {
		return github.Repo{}, errors.Wrap(err, "getting repo")
	}

	if len(repo.DefaultBranch) == 0 {
		return github.Repo{}, fmt.Errorf("default branch was nil, this is a bug on github's side")
	}

	return github.Repo{
		Owner:         repo.Owner,
		Name:          repo.Name,
		URL:           repo.CloneURL,
		DefaultBranch: repo.DefaultBranch,
		Credentials: github.AppCredentials{
			InstallationToken: installation.Token,
		},
	}, nil
}
