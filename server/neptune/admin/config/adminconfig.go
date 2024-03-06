package adminconfig

import (
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/logging"
	neptune "github.com/runatlantis/atlantis/server/neptune/temporalworker/config"
)

// Config is TerraformAdmin (admin mode) specific user config
type Config struct {
	AuthCfg          neptune.AuthConfig
	ServerCfg        neptune.ServerConfig
	FeatureConfig    neptune.FeatureConfig
	TemporalCfg      valid.Temporal
	GithubCfg        valid.Github
	TerraformCfg     neptune.TerraformConfig
	DeploymentConfig valid.StoreConfig
	JobConfig        valid.StoreConfig
	Metrics          valid.Metrics

	StatsNamespace string

	DataDir                  string
	CtxLogger                logging.Logger
	App                      githubapp.Config
	LyftAuditJobsSnsTopicArn string
}
