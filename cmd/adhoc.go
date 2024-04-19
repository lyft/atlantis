package cmd

import (
	"github.com/pkg/errors"
	cfgParser "github.com/runatlantis/atlantis/server/config"
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/legacy"
	"github.com/runatlantis/atlantis/server/logging"
	adhoc "github.com/runatlantis/atlantis/server/neptune/adhoc"
	adhocconfig "github.com/runatlantis/atlantis/server/neptune/adhoc/config"
	neptune "github.com/runatlantis/atlantis/server/neptune/temporalworker/config"
)

type Adhoc struct{}

func (a *Adhoc) NewServer(userConfig legacy.UserConfig, config legacy.Config) (ServerStarter, error) {
	ctxLogger, err := logging.NewLoggerFromLevel(userConfig.ToLogLevel())
	if err != nil {
		return nil, errors.Wrap(err, "failed to build context logger")
	}

	globalCfg := valid.NewGlobalCfg(userConfig.DataDir)
	validator := &cfgParser.ParserValidator{}

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

	cfg := &adhocconfig.Config{
		AuthCfg: neptune.AuthConfig{
			SslCertFile: userConfig.SSLCertFile,
			SslKeyFile:  userConfig.SSLKeyFile,
		},
		ServerCfg: neptune.ServerConfig{
			URL:     parsedURL,
			Version: config.AtlantisVersion,
			Port:    userConfig.Port,
		},
		TerraformCfg: neptune.TerraformConfig{
			DefaultVersion: userConfig.DefaultTFVersion,
			DownloadURL:    userConfig.TFDownloadURL,
			LogFilters:     globalCfg.TerraformLogFilter,
		},
		DataDir:          userConfig.DataDir,
		TemporalCfg:      globalCfg.Temporal,
		GithubCfg:        globalCfg.Github,
		App:              appConfig,
		CtxLogger:        ctxLogger,
		StatsNamespace:   userConfig.StatsNamespace,
		Metrics:          globalCfg.Metrics,
		GithubHostname:   userConfig.GithubHostname,
		GithubAppID:      userConfig.GithubAppID,
		GithubAppKeyFile: userConfig.GithubAppKeyFile,
		GithubAppSlug:    userConfig.GithubAppSlug,
		GlobalCfg:        globalCfg,
		GithubUser:       userConfig.GithubUser,
		GithubToken:      userConfig.GithubToken,
	}
	return adhoc.NewServer(cfg)
}
