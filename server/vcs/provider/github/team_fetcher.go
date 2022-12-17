package github

import (
	"context"
	"fmt"
	gh "github.com/google/go-github/v45/github"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"net/http"
)

type TeamMemberFetcher struct {
	ClientCreator githubapp.ClientCreator
	Org           string
}

func (t *TeamMemberFetcher) ListTeamMembers(ctx context.Context, installationToken int64, teamSlug string) ([]string, error) {
	client, err := t.ClientCreator.NewInstallationClient(installationToken)
	if err != nil {
		return nil, errors.Wrap(err, "creating installation client")
	}
	users, resp, err := client.Teams.ListTeamMembersBySlug(ctx, t.Org, teamSlug, &gh.TeamListTeamMembersOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "error fetching team members")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("not ok status fetching team members: %s", resp.Status)
	}
	var usernames []string
	for _, user := range users {
		usernames = append(usernames, user.GetLogin())
	}
	return usernames, nil
}
