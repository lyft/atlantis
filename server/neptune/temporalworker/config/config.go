package config

import (
	"github.com/hashicorp/go-version"
	"net/url"

	"github.com/palantir/go-githubapp/githubapp"
	"github.com/runatlantis/atlantis/server/core/config/valid"
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
	AppSlug  string
	Hostname string
}

// Config is TemporalWorker specific user config
type Config struct {
	AuthCfg          AuthConfig
	ServerCfg        ServerConfig
	FeatureConfig    FeatureConfig
	TemporalCfg      valid.Temporal
	TerraformCfg     TerraformConfig
	ValidationConfig ValidationConfig
	DeploymentConfig valid.StoreConfig
	JobConfig        valid.StoreConfig
	Metrics          valid.Metrics
	RevisionSetter   valid.RevisionSetter
	//TODO: combine this with above
	StatsNamespace string

	DataDir                  string
	CtxLogger                logging.Logger
	App                      githubapp.Config
	LyftAuditJobsSnsTopicArn string
}
