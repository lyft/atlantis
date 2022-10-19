package cli

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/file"
)

type Credentials struct {
	Cfg      githubapp.Config
	FileLock *file.RWLock

	once      sync.Once
	transport *ghinstallation.Transport
}

func (c *Credentials) Refresh(ctx context.Context, installationID int64) error {
	// initialize our transport once here. We don't support multiple installation ids atm
	// since we are using a global git config
	var initErr error
	c.once.Do(func() {
		transport, err := ghinstallation.New(http.DefaultTransport, c.Cfg.App.IntegrationID, installationID, []byte(c.Cfg.App.PrivateKey))
		if err != nil {
			initErr = err
			return
		}
		c.transport = transport
	})

	if initErr != nil {
		return errors.Wrap(initErr, "initializing transport")
	}

	token, err := c.transport.Token(ctx)
	if err != nil {
		return errors.Wrap(err, "refreshing token in transport")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return errors.Wrap(err, "getting home dir")
	}

	return errors.Wrap(
		c.writeCredentials(filepath.Join(home, ".git-credentials"), token),
		"writing credentials",
	)
}

func (c *Credentials) safeWriteFile(file string, contents []byte, perm os.FileMode) error {
	c.FileLock.Lock()
	defer c.FileLock.Unlock()

	return errors.Wrap(
		ioutil.WriteFile(file, contents, perm),
		"writing file",
	)
}

func (c *Credentials) safeReadFile(file string) (string, error) {
	c.FileLock.RLock()
	defer c.FileLock.RUnlock()

	contents, err := ioutil.ReadFile(file)
	if err != nil {
		return "", errors.Wrap(err, "reading file")
	}

	return string(contents), nil

}

func (c *Credentials) writeCredentials(file string, token string) error {
	toWrite := fmt.Sprintf(`https://x-access-token:%s@github.com`, token)

	// if it doesn't exist write to file
	if _, err := os.Stat(file); err != nil {
		return c.safeWriteFile(file, []byte(toWrite), os.ModePerm)
	}

	contents, err := c.safeReadFile(file)
	if err != nil {
		return errors.Wrap(err, "reading existing credentials")
	}

	// our token was refreshed so let's write it
	if contents != toWrite {
		if err := c.safeWriteFile(file, []byte(toWrite), os.ModePerm); err != nil {
			return errors.Wrap(err, "refreshing credentials file")
		}
	}

	if err := git("config", "--global", "credential.helper", "store"); err != nil {
		return err
	}

	return git("config", "--global", "url.https://x-access-token@github.com.insteadOf", "ssh://git@github.com")
}

func git(args ...string) error {
	if _, err := exec.Command("git", args...).CombinedOutput(); err != nil {
		return errors.Wrapf(err, "running git command with args %s", strings.Join(args, ","))
	}

	return nil
}
