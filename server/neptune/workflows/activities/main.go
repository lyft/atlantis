package activities

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/runatlantis/atlantis/server/neptune/lyft/feature"

	"github.com/hashicorp/go-version"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/legacy/core/runtime/cache"
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
	"github.com/slack-go/slack"
	"github.com/uber-go/tally/v4"
)

const (
	// binDirName is the name of the directory inside our data dir where
	// we download binaries.
	BinDirName = "bin"
	// TerraformPluginCacheDir is the name of the dir inside our data dir
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
	*slackActivities
}

func NewDeploy(deploymentStoreCfg valid.StoreConfig) (*Deploy, error) {
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
		// TODO: Add token once bot is created
		slackActivities: &slackActivities{Client: slack.New("",
			slack.OptionHTTPClient(http.DefaultClient))},
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

type PolicySet struct {
	Name  string
	Owner string
	Paths []string
}

func NewTerraform(tfConfig config.TerraformConfig, validationConfig config.ValidationConfig, ghAppConfig githubapp.Config, dataDir string, serverURL *url.URL, taskQueue string, streamHandler StreamCloser, opts ...TerraformOptions) (*Terraform, error) {
	binDir, err := mkSubDir(dataDir, BinDirName)
	if err != nil {
		return nil, err
	}

	cacheDir, err := mkSubDir(dataDir, TerraformPluginCacheDirName)
	if err != nil {
		return nil, err
	}
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

	tfLoader := NewTFVersionLoader(tfConfig.DownloadURL)
	if tfVersionCache == nil {
		tfVersionCache = cache.NewExecutionVersionLayeredLoadingCache(
			"terraform",
			binDir,
			tfLoader.LoadVersion,
		)
	}

	conftestLoader := ConftestVersionLoader{}
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

	defaultTfVersion, err := version.NewVersion(tfConfig.DefaultVersion)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing version %s", tfConfig.DefaultVersion)
	}
	defaultConftestVersion := validationConfig.DefaultVersion

	tfClient, err := command.NewAsyncClient(
		defaultTfVersion,
		tfVersionCache,
	)
	if err != nil {
		return nil, err
	}

	conftestClient, err := command.NewAsyncClient(
		defaultConftestVersion,
		conftestVersionCache,
	)
	if err != nil {
		return nil, err
	}

	policies := convertPolicies(validationConfig.Policies.PolicySets)

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
			Policies:               policies,
			FileValidator:          &file.Validator{},
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

func NewGithubWithClient(client githubClient, dataDir string, getter gogetter, allocator feature.Allocator) (*Github, error) {
	return &Github{
		githubActivities: &githubActivities{
			Client:      client,
			DataDir:     dataDir,
			LinkBuilder: link.Builder{},
			Getter:      getter,
			Allocator:   allocator,
		},
	}, nil
}

func NewGithub(appConfig githubapp.Config, installationID int64, scope tally.Scope, dataDir string, allocator feature.Allocator) (*Github, error) {
	clientCreator, err := githubapp.NewDefaultCachingClientCreator(
		appConfig,
		githubapp.WithClientMiddleware(
			github.ClientMetrics(scope.SubScope("app")),
		))

	if err != nil {
		return nil, errors.Wrap(err, "initializing client creator")
	}

	client := &internal.Client{
		ClientCreator:  clientCreator,
		InstallationID: installationID,
	}

	return NewGithubWithClient(client, dataDir, HashiGetter, allocator)
}

func mkSubDir(parentDir string, subDir string) (string, error) {
	fullDir := filepath.Join(parentDir, subDir)
	if err := os.MkdirAll(fullDir, 0700); err != nil {
		return "", errors.Wrapf(err, "unable to creare dir %q", fullDir)
	}

	return fullDir, nil
}

func convertPolicies(policies []valid.PolicySet) []PolicySet {
	var convertedPolicies []PolicySet
	for _, policy := range policies {
		convertedPolicies = append(convertedPolicies, PolicySet{
			Name:  policy.Name,
			Owner: policy.Owner,
			Paths: policy.Paths,
		})
	}
	return convertedPolicies
}
