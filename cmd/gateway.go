package cmd

import (
	"github.com/runatlantis/atlantis/server/legacy"
	"github.com/runatlantis/atlantis/server/neptune/gateway"
)

type GatewayCreator struct{}

func (c *GatewayCreator) NewServer(userConfig legacy.UserConfig, config legacy.Config) (ServerStarter, error) {
	// For now we just plumb this data through, ideally though we'd have gateway config pretty isolated
	// from worker config however this requires more refactoring and can be done later.
	appConfig, err := createGHAppConfig(userConfig)
	if err != nil {
		return nil, err
	}
	cfg := gateway.Config{
		DataDir:                   userConfig.DataDir,
		AutoplanFileList:          userConfig.AutoplanFileList,
		AppCfg:                    appConfig,
		RepoAllowList:             userConfig.RepoAllowlist,
		MaxProjectsPerPR:          userConfig.MaxProjectsPerPR,
		FFOwner:                   userConfig.FFOwner,
		FFRepo:                    userConfig.FFRepo,
		FFBranch:                  userConfig.FFBranch,
		FFPath:                    userConfig.FFPath,
		GithubHostname:            userConfig.GithubHostname,
		GithubWebhookSecret:       userConfig.GithubWebhookSecret,
		GithubAppID:               userConfig.GithubAppID,
		GithubAppKeyFile:          userConfig.GithubAppKeyFile,
		GithubAppSlug:             userConfig.GithubAppSlug,
		GithubStatusName:          userConfig.VCSStatusName,
		LogLevel:                  userConfig.ToLogLevel(),
		StatsNamespace:            userConfig.StatsNamespace,
		Port:                      userConfig.Port,
		RepoConfig:                userConfig.RepoConfig,
		TFDownloadURL:             userConfig.TFDownloadURL,
		SNSTopicArn:               userConfig.LyftGatewaySnsTopicArn,
		SSLKeyFile:                userConfig.SSLKeyFile,
		SSLCertFile:               userConfig.SSLCertFile,
		DefaultCheckrunDetailsURL: userConfig.DefaultCheckrunDetailsURL,
		DefaultTFVersion:          userConfig.DefaultTFVersion,
	}
	return gateway.NewServer(cfg)
}
