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
	DefaultVersion string
	DownloadURL    string
	LogFilters     valid.TerraformLogFilters
}

type ValidationConfig struct {
	DefaultVersion string
	Policies       valid.PolicySets
}

// Config is TemporalWorker specific user config
type Config struct {
	AuthCfg          AuthConfig
	ServerCfg        ServerConfig
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
