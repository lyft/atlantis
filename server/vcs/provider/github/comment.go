package github

import (
	"context"

	"github.com/google/go-github/v45/github"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
)

type CommentCreator struct {
	ClientCreator githubapp.ClientCreator
}

func (c *CommentCreator) CreateComment(ctx context.Context, installationToken int64, repo models.Repo, pullNum int, body string) error {
	client, err := c.ClientCreator.NewInstallationClient(installationToken)
	if err != nil {
		return errors.Wrap(err, "creating client")
	}

	_, _, err = client.Issues.CreateComment(ctx, repo.Owner, repo.Name, pullNum, &github.IssueComment{
		Body: github.String(body),
	})

	if err != nil {
		return errors.Wrapf(err, "creating comment with body: %s", body)
	}

	return nil
}
