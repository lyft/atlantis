package converter

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/request"
)

func Repo(external request.Repo) github.Repo {
	return github.Repo{
		Name:  external.Name,
		Owner: external.Owner,
		URL:   external.URL,
		Credentials: github.AppCredentials{
			InstallationToken: external.Credentials.InstallationToken,
		},
		DefaultBranch: external.DefaultBranch,
	}
}
