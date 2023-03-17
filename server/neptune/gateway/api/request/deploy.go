package request

import (
	"context"
	"fmt"

	"github.com/google/go-github/v45/github"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/neptune/gateway/api/middleware"
	"github.com/runatlantis/atlantis/server/neptune/gateway/api/request/external"
	"github.com/runatlantis/atlantis/server/vcs/provider/github/converter"
)

func NewDeployConverter(clientCreator githubapp.ClientCreator, repoConverter converter.RepoConverter) *JSONRequestValidationProxy[external.DeployRequest, Deploy] {
	return &JSONRequestValidationProxy[external.DeployRequest, Deploy]{
		Delegate: &DeployConverter{
			ClientCreator: clientCreator,
			RepoConverter: repoConverter,
		},
	}
}

// Deploy contains everything our deploy workflow
// needs to make this request happen.
type Deploy struct {
	RootNames         []string
	Repo              models.Repo
	Branch            string
	Revision          string
	InstallationToken int64
	User              models.User
}

type DeployConverter struct {
	githubapp.ClientCreator
	converter.RepoConverter
}

func (c *DeployConverter) Convert(ctx context.Context, r external.DeployRequest) (Deploy, error) {
	// this should be set in our auth middleware
	username := ctx.Value(middleware.UsernameContextKey)
	if username == nil {
		return Deploy{}, fmt.Errorf("user not provided")
	}

	installationToken, err := c.getInstallationToken(ctx, r.Repo.Owner)
	if err != nil {
		return Deploy{}, err
	}

	repository, branch, err := c.getRepositoryAndBranch(ctx, r, installationToken)
	if err != nil {
		return Deploy{}, err
	}

	branchName := branch.GetName()
	revision := branch.GetCommit().GetSHA()

	if len(branchName) == 0 {
		return Deploy{}, errors.Wrap(err, "branch name returned is empty, this is bug with github")
	}

	if len(revision) == 0 {
		return Deploy{}, errors.Wrap(err, "revision returned is empty, this is bug with github")
	}

	internalRepo, err := c.RepoConverter.Convert(repository)
	if err != nil {
		return Deploy{}, errors.Wrap(err, "converting repository")
	}

	return Deploy{
		Repo:              internalRepo,
		RootNames:         r.Roots,
		Branch:            branchName,
		Revision:          revision,
		InstallationToken: installationToken,
		User: models.User{
			Username: username.(string),
		},
	}, nil
}

// In order to authenticate as our GH App we need to get the organization's installation token.
func (c *DeployConverter) getInstallationToken(ctx context.Context, owner string) (int64, error) {
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

func (c *DeployConverter) getRepositoryAndBranch(ctx context.Context, r external.DeployRequest, installationToken int64) (*github.Repository, *github.Branch, error) {
	installationClient, err := c.ClientCreator.NewInstallationClient(installationToken)
	if err != nil {
		return nil, nil, errors.Wrap(err, "creating installation client")
	}

	repository, _, err := installationClient.Repositories.Get(ctx, r.Repo.Owner, r.Repo.Name)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "getting repository")
	}

	if len(repository.GetDefaultBranch()) == 0 {
		return nil, nil, fmt.Errorf("default branch was nil, this is a bug on github's side")
	}

	branch, _, err := installationClient.Repositories.GetBranch(ctx, r.Repo.Owner, r.Repo.Name, repository.GetDefaultBranch(), true)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "getting branch")
	}

	return repository, branch, nil
}
