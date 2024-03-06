package cmd

import (
	"github.com/pkg/errors"
	cfgParser "github.com/runatlantis/atlantis/server/config"
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/legacy"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/neptune/admin"
	adminconfig "github.com/runatlantis/atlantis/server/neptune/admin/config"
	neptune "github.com/runatlantis/atlantis/server/neptune/temporalworker/config"
)

type Admin struct{}

// NewServer returns the real Atlantis server object.
func (a *Admin) NewServer(userConfig legacy.UserConfig, config legacy.Config) (ServerStarter, error) {
	ctxLogger, err := logging.NewLoggerFromLevel(userConfig.ToLogLevel())
	if err != nil {
		return nil, errors.Wrap(err, "failed to build context logger")
	}

	globalCfg := valid.NewGlobalCfg(userConfig.DataDir)
	validator := &cfgParser.ParserValidator{}

	// TODO: fill in values from globalCfg
	if userConfig.RepoConfig != "" {
		globalCfg, err = validator.ParseGlobalCfg(userConfig.RepoConfig, globalCfg)
		if err != nil {
			return nil, errors.Wrapf(err, "parsing %s file", userConfig.RepoConfig)
		}
	}

	parsedURL, err := legacy.ParseAtlantisURL(userConfig.AtlantisURL)
	if err != nil {
		return nil, errors.Wrapf(err,
			"parsing atlantis url %q", userConfig.AtlantisURL)
	}

	appConfig, err := createGHAppConfig(userConfig)
	if err != nil {
		return nil, err
	}

	cfg := &adminconfig.Config{
		AuthCfg: neptune.AuthConfig{
			SslCertFile: userConfig.SSLCertFile,
			SslKeyFile:  userConfig.SSLKeyFile,
		},
		ServerCfg: neptune.ServerConfig{
			URL:     parsedURL,
			Version: config.AtlantisVersion,
			Port:    userConfig.Port,
		},
		// we need the terraformcfg stuff, since we need terraformActivities
		TerraformCfg: neptune.TerraformConfig{
			DefaultVersion: userConfig.DefaultTFVersion,
			DownloadURL:    userConfig.TFDownloadURL,
			LogFilters:     globalCfg.TerraformLogFilter,
		},
		// Do not need deployment config
		// do need datadir, we will save the archive there
		DataDir: userConfig.DataDir,
		// do need temporalconfig since we use temporal
		TemporalCfg: globalCfg.Temporal,
		// do need githubcfg, since we use github to get the archive
		GithubCfg: globalCfg.Github,
		// same as above
		App: appConfig,
		// we do need logging
		CtxLogger: ctxLogger,
		// we do need stats
		StatsNamespace: userConfig.StatsNamespace,
		// we do need metrics
		Metrics: globalCfg.Metrics,
		// no SnsTopicArn since we don't use the auditing
		// no revision setter
	}
	return admin.NewServer(cfg)
}
