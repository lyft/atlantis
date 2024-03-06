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

	appConfig, err := createGHAppConfig(userConfig)
	if err != nil {
		return nil, err
	}

	// we don't need the feature config
	cfg := &neptune.Config{
		// we need the authCfg and ssl stuff for the http server
		AuthCfg: neptune.AuthConfig{
			SslCertFile: userConfig.SSLCertFile,
			SslKeyFile:  userConfig.SSLKeyFile,
		},
		// we need the servercfg stuff, see setAtlantisURL() TODO: is this true?
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
		// also passed to terraform activities, even though we don't need conf test OPA stuff
		// TODO: But we have to introduce branching if we remove this...
		ValidationConfig: neptune.ValidationConfig{
			DefaultVersion: globalCfg.PolicySets.Version,
			Policies:       globalCfg.PolicySets,
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
	return terraformadmin.NewServer(cfg)
}
