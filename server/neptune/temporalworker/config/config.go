package config

import (
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
	DefaultVersionStr      string
	DefaultVersionFlagName string
	DownloadURL            string
	LogFilters             valid.TerraformLogFilters
}

// Config is TemporalWorker specific user config
type Config struct {
	AuthCfg               AuthConfig
	ServerCfg             ServerConfig
	TemporalCfg           valid.Temporal
	TerraformCfg          TerraformConfig
	JobStoreConfig        valid.StoreConfig
	DeploymentStoreConfig valid.StoreConfig
	Metrics               valid.Metrics
	//TODO: combine this with above
	StatsNamespace   string
	DeploymentConfig valid.Deployments

	DataDir   string
	CtxLogger logging.Logger
	App       githubapp.Config
}
