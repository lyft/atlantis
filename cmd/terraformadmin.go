package cmd

import (
	"github.com/pkg/errors"
	cfgParser "github.com/runatlantis/atlantis/server/config"
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/legacy"
	"github.com/runatlantis/atlantis/server/logging"
	neptune "github.com/runatlantis/atlantis/server/neptune/temporalworker/config"
	"github.com/runatlantis/atlantis/server/neptune/terraformadmin"
)

type TerraformAdmin struct{}

// NewServer returns the real Atlantis server object.
func (t *TerraformAdmin) NewServer(userConfig legacy.UserConfig, config legacy.Config) (ServerStarter, error) {
	ctxLogger, err := logging.NewLoggerFromLevel(userConfig.ToLogLevel())
	if err != nil {
		return nil, errors.Wrap(err, "failed to build context logger")
	}

	globalCfg := valid.NewGlobalCfg(userConfig.DataDir)
	validator := &cfgParser.ParserValidator{}

	// TODO: should terraformadminmode pass in this stuff?
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

	// TODO: we should just supply a yaml file with this info and load it directly into the
	// app config struct
	appConfig, err := createGHAppConfig(userConfig)
	if err != nil {
		return nil, err
	}

	// we don't need the feature config

	cfg := &neptune.Config{
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
		ValidationConfig: neptune.ValidationConfig{
			DefaultVersion: globalCfg.PolicySets.Version,
			Policies:       globalCfg.PolicySets,
		},
		JobConfig:                globalCfg.PersistenceConfig.Jobs,
		DeploymentConfig:         globalCfg.PersistenceConfig.Deployments,
		DataDir:                  userConfig.DataDir,
		TemporalCfg:              globalCfg.Temporal,
		GithubCfg:                globalCfg.Github,
		App:                      appConfig,
		CtxLogger:                ctxLogger,
		StatsNamespace:           userConfig.StatsNamespace,
		Metrics:                  globalCfg.Metrics,
		LyftAuditJobsSnsTopicArn: userConfig.LyftAuditJobsSnsTopicArn,
		RevisionSetter:           globalCfg.RevisionSetter,
	}
	return terraformadmin.NewServer(cfg)
}
