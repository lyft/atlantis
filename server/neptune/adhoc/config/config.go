package adhocconfig

import (
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/logging"
	neptune "github.com/runatlantis/atlantis/server/neptune/temporalworker/config"
)

// Config is TerraformAdmin (Adhoc mode) specific user config
type Config struct {
	AuthCfg       neptune.AuthConfig
	ServerCfg     neptune.ServerConfig
	FeatureConfig neptune.FeatureConfig
	TemporalCfg   valid.Temporal
	GithubCfg     valid.Github
	TerraformCfg  neptune.TerraformConfig
	Metrics       valid.Metrics

	StatsNamespace string

	DataDir   string
	CtxLogger logging.Logger
	App       githubapp.Config

	GithubAppID      int64
	GithubAppKeyFile string
	GithubAppSlug    string
	GithubHostname   string

	GlobalCfg valid.GlobalCfg
}
