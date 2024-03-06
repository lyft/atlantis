package adminconfig

import (
	"net/url"

	"github.com/hashicorp/go-version"

	"github.com/palantir/go-githubapp/githubapp"
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/logging"
)

type AuthConfig struct {
	SslCertFile string
	SslKeyFile  string
}

type ServerConfig struct {
	URL     *url.URL
	Version string
	Port    int
}

type TerraformConfig struct {
	DefaultVersion string
	DownloadURL    string
	LogFilters     valid.TerraformLogFilters
}

type ValidationConfig struct {
	DefaultVersion *version.Version
	Policies       valid.PolicySets
}

type FeatureConfig struct {
	FFOwner  string
	FFRepo   string
	FFPath   string
	FFBranch string
}

// Config is TerraformAdmin (admin mode) specific user config
type Config struct {
	AuthCfg          AuthConfig
	ServerCfg        ServerConfig
	FeatureConfig    FeatureConfig
	TemporalCfg      valid.Temporal
	GithubCfg        valid.Github
	TerraformCfg     TerraformConfig
	ValidationConfig ValidationConfig
	DeploymentConfig valid.StoreConfig
	JobConfig        valid.StoreConfig
	Metrics          valid.Metrics

	StatsNamespace string

	DataDir                  string
	CtxLogger                logging.Logger
	App                      githubapp.Config
	LyftAuditJobsSnsTopicArn string
}
