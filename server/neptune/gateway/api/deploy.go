package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/v45/github"
	"github.com/gorilla/mux"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	internalGH "github.com/runatlantis/atlantis/server/vcs/provider/github"
	"github.com/runatlantis/atlantis/server/vcs/provider/github/converter"
)

const (
	repoVarKey     = "repo"
	ownerVarKey    = "owner"
	usernameVarKey = "username"
)

type DeployRequest struct {
	Repo              models.Repo
	Branch            string
	Revision          string
	InstallationToken int64
	User              models.User
}

type DeployRequestConverter struct {
	githubapp.ClientCreator
	converter.RepoConverter
}

func (c *DeployRequestConverter) getInstallationToken(ctx context.Context, owner string) (int64, error) {
	appClient, err := c.ClientCreator.NewAppClient()
	if err != nil {
		return 0, errors.Wrap(err, "creating app client")
	}

	installation, _, err := appClient.Apps.FindOrganizationInstallation(ctx, owner)
	if err != nil {
		return 0, errors.Wrapf(err, "finding organization installation")
	}
	return installation.GetID(), nil
}

func (c *DeployRequestConverter) getRepositoryAndBranch(ctx context.Context, owner, repo string, installationToken int64) (*github.Repository, *github.Branch, error) {
	installationClient, err := c.ClientCreator.NewInstallationClient(installationToken)
	if err != nil {
		return nil, nil, errors.Wrap(err, "creating installation client")
	}

	repository, _, err := installationClient.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "getting repository")
	}

	if len(repository.GetDefaultBranch()) == 0 {
		return nil, nil, fmt.Errorf("default branch was nil, this is a bug on github's side")
	}

	branch, _, err := installationClient.Repositories.GetBranch(ctx, owner, repo, repository.GetDefaultBranch(), true)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "getting branch")
	}

	return repository, branch, nil
}

func (c *DeployRequestConverter) Convert(from *http.Request) (DeployRequest, error) {
	ctx := from.Context()
	vars := mux.Vars(from)

	owner, ok := vars[ownerVarKey]
	if !ok {
		return DeployRequest{}, fmt.Errorf("owner not provided")
	}

	repo, ok := vars[repoVarKey]
	if !ok {
		return DeployRequest{}, fmt.Errorf("repo not provided")
	}

	user, ok := vars[usernameVarKey]
	if !ok {
		return DeployRequest{}, fmt.Errorf("username not provided")
	}

	installationToken, err := c.getInstallationToken(ctx, owner)
	if err != nil {
		return DeployRequest{}, err
	}

	repository, branch, err := c.getRepositoryAndBranch(ctx, owner, repo, installationToken)
	if err != nil {
		return DeployRequest{}, err
	}

	branchName := branch.GetName()
	revision := branch.GetCommit().GetSHA()

	if len(branchName) == 0 {
		return DeployRequest{}, errors.Wrap(err, "branch name returned is empty, this is bug with github")
	}

	if len(revision) == 0 {
		return DeployRequest{}, errors.Wrap(err, "revision returned is empty, this is bug with github")
	}

	internalRepo, err := c.RepoConverter.Convert(repository)
	if err != nil {
		return DeployRequest{}, errors.Wrap(err, "converting repository")
	}

	return DeployRequest{
		Repo:              internalRepo,
		Branch:            branchName,
		Revision:          revision,
		InstallationToken: installationToken,
		User: models.User{
			Username: user,
		},
	}, nil

}

type DeployHandler struct {
	Deployer event.RootDeployer
}

func (c *DeployHandler) Handle(ctx context.Context, request DeployRequest) error {
	// make this async
	return c.Deployer.Deploy(ctx, event.RootDeployOptions{

		// fetch this from github
		Repo:     request.Repo,
		Branch:   request.Branch,
		Revision: request.Revision,

		// this we won't have, maybe we can add some gh auth to add this
		Sender: request.User,

		// get from github
		InstallationToken: request.InstallationToken,

		BuilderOptions: event.BuilderOptions{
			RepoFetcherOptions: internalGH.RepoFetcherOptions{
				CloneDepth: 1,
			},

			FileFetcherOptions: internalGH.FileFetcherOptions{
				Sha: request.Revision,
			},
		},

		Trigger: workflows.ManualTrigger,
	})
}
