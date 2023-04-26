// Copyright 2017 HootSuite Media Inc.
//
// Licensed under the Apache License, Version 2.0 (the License);
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an AS IS BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// Modified hereafter by contributors to runatlantis/atlantis.

package cmd

import (
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/alecthomas/kong"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server"
	cfgParser "github.com/runatlantis/atlantis/server/core/config"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/vcs/bitbucketcloud"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/neptune/gateway"
	"github.com/runatlantis/atlantis/server/neptune/temporalworker"
	neptune "github.com/runatlantis/atlantis/server/neptune/temporalworker/config"
)

type Context struct {
	Version string
}

type VersionCmd struct{}

func (cmd *VersionCmd) Run(ctx Context) error {
	fmt.Printf("atlantis %s\n", ctx.Version)
	return nil
}

type ServerCmd struct {
	server.UserConfig `kong:"embed"`
}

var CLI struct {
	Version VersionCmd `cmd:"version" help:"Print the current Atlantis version"`
	Server  ServerCmd  `cmd:"server" help:"Start the atlantis server"`
}

var FlagsVars = kong.Vars{
	"help_ad_token": "Azure DevOps token of API user.",
	"help_ad_user":  "Azure DevOps username of API user.",
	"help_ad_webhook_password": "Azure DevOps basic HTTP authentication password for inbound webhooks " +
		"(see https://docs.microsoft.com/en-us/azure/devops/service-hooks/authorize?view=azure-devops).\n" +
		"SECURITY WARNING: If not specified, Atlantis won't be able to validate that the incoming webhook " +
		"call came from your Azure DevOps org. This means that an attacker could spoof calls to Atlantis " +
		"and cause it to perform malicious actions.",
	"help_ad_webhook_user": "Azure DevOps basic HTTP authentication username for inbound webhooks.",
	"help_atlantis_url": "URL that Atlantis can be reached at. Defaults to http://$(hostname):$port. " +
		"Supports a base path ex. https://example.com/basepath.",
	"help_autoplan_file_list": "Comma separated list of file patterns that Atlantis will use to check if " +
		"a directory contains modified files that should trigger project planning. Patterns use the " +
		"dockerignore (https://docs.docker.com/engine/reference/builder/#dockerignore_file) syntax." +
		"Use single quotes to avoid shell expansion of '*'. A custom Workflow that uses autoplan " +
		"'when_modified' will ignore this value.",
	"default_autoplan_file_list": "**/*.tf,**/*.tfvars,**/*.tfvars.json,**/terragrunt.hcl",
	"help_bitbucket_user":        "Bitbucket username of API user.",
	"help_bitbucket_token":       "Bitbucket app password of API user.",
	"help_bitbucket_base_url": "Base URL of Bitbucket Server (aka Stash) installation. " +
		"Must include 'http://' or 'https://'. If using Bitbucket Cloud (bitbucket.org), do not set.",
	"default_bitbucket_base_url": bitbucketcloud.BaseURL,
	"help_bitbucket_webhook_secret": "Secret used to validate Bitbucket webhooks. Only Bitbucket Server " +
		"supports webhook secrets.\n" +
		"SECURITY WARNING: If not specified, Atlantis won't be able to validate that the incoming webhook " +
		"call came from Bitbucket. This means that an attacker could spoof calls to Atlantis and cause it " +
		"to perform malicious actions.",
	"help_checkout_strategy": "How to check out pull requests. Accepts either 'branch' (default) or 'merge'. " +
		"If set to branch, Atlantis will check out the source branch of the pull request. If set to merge, " +
		"Atlantis will check out the destination branch of the pull request (ex. master) and then locally " +
		"perform a git merge of the source branch. This effectively means Atlantis operates on the repo " +
		"as it will look after the pull request is merged.",
	"help_config":                       "Path to yaml config file where flag values can also be set.",
	"help_data_dir":                     "Path to directory to store Atlantis data.",
	"help_default_checkrun_details_url": "URL to redirect to if details URL is not set in the checkrun UI.",
	"help_ff_owner":                     "Owner of the repo used to house feature flag configuration.",
	"help_ff_repo":                      "Repo used to house feature flag configuration.",
	"help_ff_branch":                    "Branch on repo to pull the feature flag configuration.",
	"help_ff_path":                      "Path in repo to get feature flag configuration.",
	"help_gh_hostname": "Hostname of your Github Enterprise installation. " +
		"If using github.com, no need to set.",
	"help_gh_user":         "GitHub username of API user.",
	"help_gh_token":        "GitHub token of API user.",
	"help_gh_app_key":      "The GitHub App's private key",
	"help_gh_app_key_file": "A path to a file containing the GitHub App's private key.",
	"help_gh_app_slug":     "The Github app slug (ie. the URL-friendly name of your GitHub App).",
	"help_gh_organization": "The name of the GitHub organization to use during the creation " +
		"of a Github App for Atlantis.",
	"help_gh_webhook_secret": "Secret used to validate GitHub webhooks " +
		"(see https://developer.github.com/webhooks/securing/).\n" +
		"SECURITY WARNING: If not specified, Atlantis won't be able to validate that the incoming webhook call " +
		"came from GitHub. This means that an attacker could spoof calls to Atlantis " +
		"and cause it to perform malicious actions.",
	"help_gitlab_hostname": "Hostname of your GitLab Enterprise installation. If using gitlab.com, no need to set.",
	"help_gitlab_user":     "GitLab username of API user.",
	"help_gitlab_token":    "GitLab token of API user.",
	"help_gitlab_webhook_secret": "Optional secret used to validate GitLab webhooks.\n" +
		"SECURITY WARNING: If not specified, Atlantis won't be able to validate that the " +
		"incoming webhook call came from GitLab. This means that an attacker could spoof calls to Atlantis " +
		"and cause it to perform malicious actions.",
	"help_log_level":       "Log level. Either debug, info, warn, or error.",
	"help_stats_namespace": "Namespace for aggregating stats.",
	"help_repo_config": "Path to a repo config file, used to customize how Atlantis runs on each repo. " +
		"See runatlantis.io/docs for more details.",
	"help_repo_config_json": "Specify repo config as a JSON string. Useful if you don't want " +
		"to write a config file to disk.",
	"help_repo_allowlist": "Comma separated list of repositories that Atlantis will operate on." +
		"The format is {hostname}/{owner}/{repo}, ex. github.com/runatlantis/atlantis. '*' matches any " +
		"characters until the next comma. Examples:\n" +
		"all repos: '*' (not secure),\n" +
		"an entire hostname: 'internalgithub.com/*' or\n" +
		"an organization: 'github.com/runatlantis/*'.\n" +
		"For Bitbucket Server, {owner} is the name of the project (not the key).",
	"help_slack_token": "API token for Slack notifications.",
	"help_ssl_cert_file": "File containing x509 Certificate used for serving HTTPS. " +
		"If the cert is signed by a CA, the file should be the concatenation of the server's certificate, " +
		"any intermediates, and the CA's certificate.",
	"help_ssl_key_file":    "File containing x509 private key.",
	"help_tf_download_url": "Base URL to download Terraform versions from.",
	"help_default_tf_version": "Terraform version to default to (ex. v0.12.0). Will download if not yet on disk. " +
		"If not set, Atlantis uses the terraform binary in its PATH.",
	"help_vcs_status_name": "Name used to identify Atlantis for pull request statuses.",
	"help_lyft_audit_jobs_sns_topic_arn": "Provide SNS topic ARN to publish apply workflow's status. " +
		"Sns topic is used for auditing purposes",
	"help_lyft_gateway_sns_topic_arn": "Provide SNS topic ARN to publish GH events to atlantis worker. " +
		"Sns topic is used in gateway proxy mode",
	"help_lyft_mode": "Specifies which mode to run atlantis in. If not set, will assume the default mode. " +
		"Available modes:\n" +
		"default: Runs atlantis with default event handler that processes events within same.\n" +
		"gateway: Runs atlantis with gateway event handler that publishes events through sns.\n" +
		"worker:  Runs atlantis with a sqs handler that polls for events in the queue to process.\n" +
		"hybrid:  Runs atlantis with both a gateway event handler and sqs handler to perform " +
		"both gateway and worker behaviors.",
	"help_lyft_worker_queue_url": "Provide queue of AWS SQS queue for atlantis work to pull GH events from and process.",
	"help_disable_apply_all": "Disable \"atlantis apply\" command without any flags (i.e. apply all). " +
		"A specific project/workspace/directory has to be specified for applies.",
	"help_disable_apply":    "Disable all \"atlantis apply\" command regardless of which flags are passed with it.",
	"help_disable_autoplan": "Disable atlantis auto planning feature",
	"help_enable_regexp_cmd": "Enable Atlantis to use regular expressions on plan/apply commands " +
		"when \"-p\" flag is passed with it.",
	"help_enable_diff_markdown_format": "Enable Atlantis to format Terraform plan output into " +
		"a markdown-diff friendly format for color-coding purposes.",
	"help_enable_policy_checks": "Blocks applies on plans that fail any of the defined conftest policies",
	"help_allow_draft_prs":      "Enable autoplan for Github Draft Pull Requests",
	"help_hide_prev_plan_comments": "Hide previous plan comments to reduce clutter in the PR. " +
		"VCS support is limited to: GitHub.",
	"help_disable_markdown_folding": "Toggle off folding in markdown output.",
	"help_write_git_creds": "Write out a .git-credentials file with the provider user and token to allow " +
		"cloning private modules over HTTPS or SSH. " +
		"This writes secrets to disk and should only be enabled in a secure environment.",
	"help_parallel_pool_size":     "Max size of the wait group that runs parallel plans and applies (if enabled).",
	"help_max_projects_per_pr":    "Max number of projects to operate on in a given pull request.",
	"default_max_projects_per_pr": strconv.Itoa(events.InfiniteProjectsPerPR),
	"help_port":                   "Port to bind to.",
	"help_gh_app_id":              "GitHub App Id. If defined, initializes the GitHub client with app-based credentials",
}

func (cmd *ServerCmd) Validate() error {
	if cmd.UserConfig.GithubSecrets.AppID == 0 &&
		string(cmd.UserConfig.GithubSecrets.User) == "" &&
		string(cmd.UserConfig.GitlabSecrets.User) == "" &&
		string(cmd.UserConfig.BitbucketSecrets.User) == "" &&
		string(cmd.UserConfig.AzureDevopsSecrets.User) == "" {
		return fmt.Errorf("credentials for at least one VCS provider should be defined")
	}

	var err error

	err = cmd.UserConfig.SSLSecrets.Validate()
	if err != nil {
		return err
	}
	err = cmd.UserConfig.AzureDevopsSecrets.Validate()
	if err != nil {
		return err
	}
	err = cmd.UserConfig.BitbucketSecrets.Validate()
	if err != nil {
		return err
	}
	err = cmd.UserConfig.GithubSecrets.Validate()
	if err != nil {
		return err
	}
	err = cmd.UserConfig.GitlabSecrets.Validate()
	if err != nil {
		return err
	}
	if cmd.UserConfig.RepoConfig != "" && cmd.UserConfig.RepoConfigJSON != "" {
		return fmt.Errorf("cannot set both path to repo config and repo config json at the same time")
	}

	return nil
}

func (cmd *ServerCmd) Run(ctx Context) error {
	var err error
	cmd.AtlantisVersion = ctx.Version
	if cmd.UserConfig.AtlantisURL.String() == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return errors.Wrap(err, "failed to determine hostname")
		}
		atlantisUrl, err := url.Parse(fmt.Sprintf("http://%s:%d", hostname, cmd.Port))
		if err != nil {
			return err
		}
		cmd.UserConfig.AtlantisURL = server.HttpUrl{atlantisUrl}
	}
	// Legacy code still partially supports other VCS configs
	// so GithubAppKeyFile needs to exist to create the githubapp config
	appConfig := githubapp.Config{}
	if cmd.GithubSecrets.AppKeyFile != "" {
		appConfig, err = cmd.createGHAppConfig()
		if err != nil {
			return err
		}
	}

	// Config looks good. Start the server.
	if err := os.MkdirAll(cmd.DataDir, 0700); err != nil {
		return err
	}
	var srv Server
	switch cmd.LyftMode {
	case server.Gateway:
		srv, err = cmd.NewGatewayServer(appConfig)
	case server.TemporalWorker:
		srv, err = cmd.NewTemporalWorkerServer(appConfig)
	default:
		srv, err = cmd.NewWorkerServer(appConfig)
	}
	if err != nil {
		return errors.Wrap(err, "initializing server")
	}

	return srv.Start()
}

type Server interface {
	Start() error
}

func (cmd *ServerCmd) createGHAppConfig() (githubapp.Config, error) {
	return githubapp.Config{
		App: struct {
			IntegrationID int64  "yaml:\"integration_id\" json:\"integrationId\""
			WebhookSecret string "yaml:\"webhook_secret\" json:\"webhookSecret\""
			PrivateKey    string "yaml:\"private_key\" json:\"privateKey\""
		}{
			IntegrationID: cmd.UserConfig.GithubSecrets.AppID,
			WebhookSecret: cmd.UserConfig.GithubSecrets.WebhookSecret,
			PrivateKey:    cmd.UserConfig.GithubSecrets.AppKeyFile,
		},

		//TODO: parameterize this
		WebURL:   "https://github.com",
		V3APIURL: "https://api.github.com",
		V4APIURL: "https://api.github.com/graphql",
	}, nil
}

func (cmd *ServerCmd) NewGatewayServer(appCfg githubapp.Config) (Server, error) {
	// For now we just plumb this data through, ideally though we'd have gateway config pretty isolated
	// from worker config however this requires more refactoring and can be done later.

	cfg := cmd.UserConfig
	allowlist := []string{}
	for _, expr := range cfg.RepoAllowlist {
		allowlist = append(allowlist, expr.String())
	}

	neptuneCfg := gateway.Config{
		DataDir:                   cfg.DataDir,
		AutoplanFileList:          cfg.AutoplanFileList.PatternMatcher,
		AppCfg:                    appCfg,
		RepoAllowlist:             allowlist,
		MaxProjectsPerPR:          cfg.MaxProjectsPerPR,
		FFOwner:                   cfg.FFOwner,
		FFRepo:                    cfg.FFRepo,
		FFBranch:                  cfg.FFBranch,
		FFPath:                    cfg.FFPath,
		GithubHostname:            cfg.GithubSecrets.Hostname.String(),
		GithubWebhookSecret:       cfg.GithubSecrets.WebhookSecret,
		GithubAppID:               cfg.GithubSecrets.AppID,
		GithubAppKeyFile:          cfg.GithubSecrets.AppKeyFile,
		GithubAppSlug:             cfg.GithubSecrets.AppSlug,
		GithubStatusName:          cfg.VCSStatusName,
		LogLevel:                  cfg.LogLevel,
		StatsNamespace:            cfg.StatsNamespace,
		Port:                      cfg.Port,
		RepoConfig:                cfg.RepoConfig,
		TFDownloadURL:             cfg.TFDownloadURL.String(),
		SNSTopicArn:               cfg.LyftGatewaySnsTopicArn,
		SSLKeyFile:                cfg.SSLSecrets.KeyFile,
		SSLCertFile:               cfg.SSLSecrets.CertFile,
		DefaultCheckrunDetailsURL: cfg.DefaultCheckrunDetailsURL.String(),
	}
	return gateway.NewServer(neptuneCfg)
}

// NewWorkerServer returns the real Atlantis server object.
func (cmd *ServerCmd) NewWorkerServer(appCfg githubapp.Config) (Server, error) {
	return server.NewServer(cmd.UserConfig, appCfg)
}

// NewServer returns the real Atlantis server object.
func (cmd *ServerCmd) NewTemporalWorkerServer(appCfg githubapp.Config) (Server, error) {
	cfg := cmd.UserConfig
	ctxLogger, err := logging.NewLoggerFromLevel(cfg.LogLevel)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build context logger")
	}

	globalCfg := valid.NewGlobalCfg(cfg.DataDir)
	validator := &cfgParser.ParserValidator{}
	if cfg.RepoConfig != "" {
		globalCfg, err = validator.ParseGlobalCfg(cfg.RepoConfig, globalCfg)
		if err != nil {
			return nil, errors.Wrapf(err, "parsing %s file", cfg.RepoConfig)
		}
	}

	neptuneCfg := &neptune.Config{
		AuthCfg: neptune.AuthConfig{
			SslCertFile: cfg.SSLSecrets.CertFile,
			SslKeyFile:  cfg.SSLSecrets.KeyFile,
		},
		ServerCfg: neptune.ServerConfig{
			URL:     cfg.AtlantisURL.URL,
			Version: cfg.AtlantisVersion,
			Port:    cfg.Port,
		},
		TerraformCfg: neptune.TerraformConfig{
			DefaultVersion: cfg.DefaultTFVersion,
			DownloadURL:    cfg.TFDownloadURL.String(),
			LogFilters:     globalCfg.TerraformLogFilter,
		},
		ValidationConfig: neptune.ValidationConfig{
			DefaultVersion: globalCfg.PolicySets.Version,
			Policies:       globalCfg.PolicySets,
		},
		JobConfig:                globalCfg.PersistenceConfig.Jobs,
		DeploymentConfig:         globalCfg.PersistenceConfig.Deployments,
		DataDir:                  cfg.DataDir,
		TemporalCfg:              globalCfg.Temporal,
		App:                      appCfg,
		CtxLogger:                ctxLogger,
		StatsNamespace:           cfg.StatsNamespace,
		Metrics:                  globalCfg.Metrics,
		LyftAuditJobsSnsTopicArn: cfg.LyftAuditJobsSnsTopicArn,
		RevisionSetter:           globalCfg.RevisionSetter,
	}
	return temporalworker.NewServer(neptuneCfg)
}
