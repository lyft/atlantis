package activities

import (
	"os"
	"path/filepath"

	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/terraform"
	"github.com/runatlantis/atlantis/server/jobs"
	"github.com/runatlantis/atlantis/server/neptune/config"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities/github"
	"github.com/uber-go/tally/v4"
)

const (
	// binDirName is the name of the directory inside our data dir where
	// we download binaries.
	BinDirName = "bin"
	// terraformPluginCacheDir is the name of the dir inside our data dir
	// where we tell terraform to cache plugins and modules.
	TerraformPluginCacheDirName = "plugin-cache"
)

// Exported Activites should be here.
// The convention should be one exported struct per workflow
// This guarantees function naming uniqueness within a given workflow
// which is a requirement at a per worker level
//
// Note: This doesn't prevent issues with naming duplication that can come up when
// registering multiple workflows to the same worker
type Deploy struct {
	*dbActivities
}

func NewDeploy(config githubapp.Config, scope tally.Scope) (*Deploy, error) {
	return &Deploy{
		dbActivities: &dbActivities{},
	}, nil
}

type Terraform struct {
	*terraformActivities
	*executeCommandActivities
	*notifyActivities
	*cleanupActivities
}

func NewTerraform(config config.TerraformConfig, scope tally.Scope, prjCmdOutputHandler jobs.ProjectCommandOutputHandler) (*Terraform, error) {
	binDir, err := mkSubDir(config.DataDir, BinDirName)
	if err != nil {
		return nil, err
	}

	cacheDir, err := mkSubDir(config.DataDir, TerraformPluginCacheDirName)
	if err != nil {
		return nil, err
	}

	tfVersion, err := terraform.GetDefaultVersion(config.DefaultVersionStr, config.DefaultVersionFlagName)
	if err != nil {
		return nil, err
	}

	tfClient, err := terraform.NewClient(
		binDir,
		cacheDir,
		config.DefaultVersionStr,
		config.DefaultVersionFlagName,
		config.DownloadURL,
		&terraform.DefaultDownloader{},
		true,
		prjCmdOutputHandler,
	)
	if err != nil {
		return nil, err
	}

	return &Terraform{
		executeCommandActivities: &executeCommandActivities{},
		terraformActivities: &terraformActivities{
			TerraformExecutor: tfClient,
			DefaultTFVersion:  tfVersion,
			Scope:             scope.SubScope("terraform"),
		},
	}, nil
}

type Github struct {
	*githubActivities
}

func NewGithub(config githubapp.Config, scope tally.Scope) (*Github, error) {
	clientCreator, err := githubapp.NewDefaultCachingClientCreator(
		config,
		githubapp.WithClientMiddleware(
			github.ClientMetrics(scope.SubScope("app")),
		))

	if err != nil {
		return nil, errors.Wrap(err, "initializing client creator")
	}
	return &Github{
		githubActivities: &githubActivities{
			ClientCreator: clientCreator,
		},
	}, nil
}

func mkSubDir(parentDir string, subDir string) (string, error) {
	fullDir := filepath.Join(parentDir, subDir)
	if err := os.MkdirAll(fullDir, 0700); err != nil {
		return "", errors.Wrapf(err, "unable to creare dir %q", fullDir)
	}

	return fullDir, nil
}
