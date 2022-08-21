package source

import (
	"fmt"
	"github.com/runatlantis/atlantis/server/events"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/logging"
)

// GithubAppWorkingDir implements TmpWorkingDir.
// It acts as a proxy to an instance of TmpWorkingDir that refreshes the app's token
// before every clone, given Github App tokens expire quickly
type GithubAppWorkingDir struct {
	TmpWorkingDir
	Credentials    vcs.GithubCredentials
	GithubHostname string
	Logger         logging.Logger
}

// Clone writes a fresh token for Github App authentication
func (g *GithubAppWorkingDir) Clone(baseRepo models.Repo, sha string, destinationPath string) error {
	token, err := g.Credentials.GetToken()
	if err != nil {
		return errors.Wrap(err, "getting github token")
	}

	home, err := homedir.Dir()
	if err != nil {
		return errors.Wrap(err, "getting home dir to write ~/.git-credentials file")
	}

	// https://developer.github.com/apps/building-github-apps/authenticating-with-github-apps/#http-based-git-access-by-an-installation
	if err := events.WriteGitCreds("x-access-token", token, g.GithubHostname, home, g.Logger, true); err != nil {
		return err
	}
	// Realistically, this is a super brittle way of supporting clones using gh app installation tokens
	// This URL should be built during Repo creation and the struct should be immutable going forward.
	// Doing this requires a larger refactor however, and can probably be coupled with supporting > 1 installation
	authURL := fmt.Sprintf("://x-access-token:%s", token)
	baseRepo.CloneURL = strings.Replace(baseRepo.CloneURL, "://:", authURL, 1)
	baseRepo.SanitizedCloneURL = strings.Replace(baseRepo.SanitizedCloneURL, "://:", "://x-access-token:", 1)

	return g.TmpWorkingDir.Clone(baseRepo, sha, destinationPath)
}
