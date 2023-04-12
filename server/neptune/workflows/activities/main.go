package activities

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-version"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/core/runtime/cache"
	legacy_tf "github.com/runatlantis/atlantis/server/core/terraform"
	"github.com/runatlantis/atlantis/server/neptune/storage"
	"github.com/runatlantis/atlantis/server/neptune/temporalworker/config"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/command"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/deployment"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/file"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	internal "github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github/cli"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github/link"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/uber-go/tally/v4"
)

const (
	// binDirName is the name of the directory inside our data dir where
	// we download binaries.
	BinDirName = "bin"
	// PluginCacheDir is the name of the dir inside our data dir
	// where we tell commands to cache plugins and modules.
	PluginCacheDirName = "plugin-cache"
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
	*auditActivities
}

func NewDeploy(deploymentStoreCfg valid.StoreConfig, snsWriter io.Writer) (*Deploy, error) {
	storageClient, err := storage.NewClient(deploymentStoreCfg)
	if err != nil {
		return nil, errors.Wrap(err, "intializing stow client")
	}

	deploymentStore, err := deployment.NewStore(storageClient)
	if err != nil {
		return nil, errors.Wrap(err, "initializing deployment info store")
	}

	return &Deploy{
		dbActivities: &dbActivities{
			DeploymentInfoStore: deploymentStore,
		},
		auditActivities: &auditActivities{
			SnsWriter: snsWriter,
		},
	}, nil
}

type Terraform struct {
	*terraformActivities
	*conftestActivity
	*executeCommandActivities
	*workerInfoActivity
	*cleanupActivities
	*jobActivities
}

type StreamCloser interface {
	streamer
	closer
}

type TerraformOptions struct {
	TFVersionCache          cache.ExecutionVersionCache
	ConftestVersionCache    cache.ExecutionVersionCache
	GitCredentialsRefresher gitCredentialsRefresher
}

func NewTerraform(config config.TerraformConfig, ghAppConfig githubapp.Config, dataDir string, serverURL *url.URL, taskQueue string, streamHandler StreamCloser, opts ...TerraformOptions) (*Terraform, error) {
	binDir, err := mkSubDir(dataDir, BinDirName)
	if err != nil {
		return nil, err
	}

	cacheDir, err := mkSubDir(dataDir, PluginCacheDirName)
	if err != nil {
		return nil, err
	}
	downloader := &legacy_tf.DefaultDownloader{}
	gitCredentialsFileLock := &file.RWLock{}

	var tfVersionCache cache.ExecutionVersionCache
	var conftestVersionCache cache.ExecutionVersionCache
	var credentialsRefresher gitCredentialsRefresher
	for _, o := range opts {
		if o.TFVersionCache != nil {
			tfVersionCache = o.TFVersionCache
		}

		if o.ConftestVersionCache != nil {
			conftestVersionCache = o.ConftestVersionCache
		}

		if credentialsRefresher != nil {
			credentialsRefresher = o.GitCredentialsRefresher
		}
	}

	tfLoader := legacy_tf.NewVersionLoader(downloader, config.TFDownloadURL)
	if tfVersionCache == nil {
		tfVersionCache = cache.NewExecutionVersionLayeredLoadingCache(
			"terraform",
			binDir,
			tfLoader.LoadVersion,
		)
	}

	conftestLoader := legacy_tf.NewVersionLoader(downloader, config.ConftestDownloadURL)
	if conftestVersionCache == nil {
		conftestVersionCache = cache.NewExecutionVersionLayeredLoadingCache(
			"conftest",
			binDir,
			conftestLoader.LoadVersion,
		)
	}

	if credentialsRefresher == nil {
		credentialsRefresher, err = cli.NewCredentials(ghAppConfig, gitCredentialsFileLock)
		if err != nil {
			return nil, errors.Wrap(err, "initializing credentials")
		}
	}

	tfClient, err := command.NewAsyncClient(
		config.TFDefaultVersion,
		tfVersionCache,
	)
	if err != nil {
		return nil, err
	}

	conftestClient, err := command.NewAsyncClient(
		config.ConftestDefaultVersion,
		conftestVersionCache,
	)
	if err != nil {
		return nil, err
	}

	defaultTfVersion, err := version.NewVersion(config.TFDefaultVersion)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing version %s", config.TFDefaultVersion)
	}
	defaultConftestVersion, err := version.NewVersion(config.ConftestDefaultVersion)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing version %s", config.ConftestDefaultVersion)
	}
	return &Terraform{
		executeCommandActivities: &executeCommandActivities{},
		workerInfoActivity: &workerInfoActivity{
			ServerURL: serverURL,
			TaskQueue: taskQueue,
		},
		terraformActivities: &terraformActivities{
			TerraformClient:        tfClient,
			StreamHandler:          streamHandler,
			DefaultTFVersion:       defaultTfVersion,
			GitCLICredentials:      credentialsRefresher,
			GitCredentialsFileLock: gitCredentialsFileLock,
			FileWriter:             &file.Writer{},
			CacheDir:               cacheDir,
		},
		conftestActivity: &conftestActivity{
			DefaultConftestVersion: defaultConftestVersion,
			ConftestClient:         conftestClient,
			StreamHandler:          streamHandler,
		},
		jobActivities: &jobActivities{
			StreamCloser: streamHandler,
		},
	}, nil
}

type Github struct {
	*githubActivities
}

type LinkBuilder interface {
	BuildDownloadLinkFromArchive(archiveURL *url.URL, root terraform.Root, repo internal.Repo, revision string) string
}

func NewGithubWithClient(client githubClient, dataDir string, getter gogetter) (*Github, error) {
	return &Github{
		githubActivities: &githubActivities{
			Client:      client,
			DataDir:     dataDir,
			LinkBuilder: link.Builder{},
			Getter:      getter,
		},
	}, nil
}

func NewGithub(appConfig githubapp.Config, scope tally.Scope, dataDir string) (*Github, error) {
	clientCreator, err := githubapp.NewDefaultCachingClientCreator(
		appConfig,
		githubapp.WithClientMiddleware(
			github.ClientMetrics(scope.SubScope("app")),
		))

	if err != nil {
		return nil, errors.Wrap(err, "initializing client creator")
	}

	client := &internal.Client{
		ClientCreator: clientCreator,
	}

	return NewGithubWithClient(client, dataDir, HashiGetter)
}

func mkSubDir(parentDir string, subDir string) (string, error) {
	fullDir := filepath.Join(parentDir, subDir)
	if err := os.MkdirAll(fullDir, 0700); err != nil {
		return "", errors.Wrapf(err, "unable to creare dir %q", fullDir)
	}

	return fullDir, nil
}

type RevsionSetter struct {
	*prRevisionSetterActivities
}

func NewRevisionSetter(cfg valid.RevisionSetter) (*RevsionSetter, error) {
	// Use a NoopClient if revision setter is not configured
	var client revisionSetterClient
	if cfg.URL == "" {
		client = &NoopClient{}
	} else {
		client = &http.Client{}
	}

	return NewRevisionSetterWithClient(client, cfg)
}

func NewRevisionSetterWithClient(client revisionSetterClient, cfg valid.RevisionSetter) (*RevsionSetter, error) {
	return &RevsionSetter{
		prRevisionSetterActivities: &prRevisionSetterActivities{
			client:    client,
			url:       cfg.URL,
			basicAuth: cfg.BasicAuth,
		},
	}, nil
}
