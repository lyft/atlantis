package server

import (
	"fmt"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/alecthomas/kong"
	kongyaml "github.com/alecthomas/kong-yaml"
	"github.com/docker/docker/pkg/fileutils"
	"github.com/runatlantis/atlantis/server/events/vcs/bitbucketcloud"
	"github.com/runatlantis/atlantis/server/logging"
)

type ConfigFlag string

func (c ConfigFlag) BeforeResolve(kongCli *kong.Kong, ctx *kong.Context, trace *kong.Path) error {
	path := string(ctx.FlagValue(trace.Flag).(ConfigFlag))
	if path == "" {
		return nil
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		kong.Configuration(kongyaml.Loader).Apply(kongCli)
	case ".json":
		kong.Configuration(kong.JSON).Apply(kongCli)
	default:
		return fmt.Errorf("no loader for config with extension %q found", ext)
	}
	resolver, err := kongCli.LoadConfig(path)
	if err != nil {
		return err
	}
	ctx.AddResolver(resolver)
	return nil
}

// URL with http or https schema
type HttpUrl struct {
	*url.URL
}

func (h *HttpUrl) Decode(ctx *kong.DecodeContext) error {
	var rawUrl string
	err := ctx.Scan.PopValueInto("string", &rawUrl)
	if err != nil {
		return err
	}
	parsedUrl, err := url.Parse(rawUrl)
	if err != nil {
		return err
	}
	if parsedUrl.Scheme != "http" && parsedUrl.Scheme != "https" {
		return fmt.Errorf("failed to parse HTTP url: protocol %q is not supported", parsedUrl.Scheme)
	}
	*h = HttpUrl{parsedUrl}
	return nil
}

func (h *HttpUrl) String() string {
	if h != nil && h.URL != nil {
		return h.URL.String()
	}
	return ""
}

// URL object without schema
type Schemeless struct {
	*url.URL
}

func (s *Schemeless) Decode(ctx *kong.DecodeContext) error {
	var rawUrl string
	err := ctx.Scan.PopValueInto("string", &rawUrl)
	if err != nil {
		return err
	}
	parsedUrl, err := url.Parse(rawUrl)
	if err != nil {
		return err
	}
	if parsedUrl.Host == "" {
		parsedUrl, err = url.Parse(fmt.Sprintf("//%s", rawUrl))
		if err != nil {
			return err
		}
	}
	parsedUrl.Scheme = ""
	*s = Schemeless{parsedUrl}
	return nil
}

func (s *Schemeless) String() string {
	if s != nil && s.URL != nil {
		return s.URL.String()
	}
	return ""
}

// Pattern path matcher
type Matcher struct {
	*fileutils.PatternMatcher
}

func (m *Matcher) Decode(ctx *kong.DecodeContext) error {
	var rawFilesList string
	err := ctx.Scan.PopValueInto("string", &rawFilesList)
	if err != nil {
		return err
	}
	patternMatcher, err := fileutils.NewPatternMatcher(strings.Split(rawFilesList, ","))
	if err != nil {
		return err
	}
	*m = Matcher{patternMatcher}
	return nil
}

// VCS user
type User string

func (u *User) Decode(ctx *kong.DecodeContext) error {
	var rawUser string
	err := ctx.Scan.PopValueInto("string", &rawUser)
	if err != nil {
		return err
	}
	*u = User(strings.TrimPrefix(rawUser, "@"))
	return nil
}

// Lyft Mode
type Mode int

const (
	Default Mode = iota
	Gateway
	Worker
	TemporalWorker
)

func (m *Mode) Decode(ctx *kong.DecodeContext) error {
	var rawMode string
	err := ctx.Scan.PopValueInto("string", &rawMode)
	if err != nil {
		return err
	}
	switch strings.ToLower(rawMode) {
	case "default":
		ctx.Value.Target.Set(reflect.ValueOf(Default))
	case "gateway":
		ctx.Value.Target.Set(reflect.ValueOf(Gateway))
	case "worker":
		ctx.Value.Target.Set(reflect.ValueOf(Worker))
	case "temporalworker":
		ctx.Value.Target.Set(reflect.ValueOf(TemporalWorker))
	default:
		return fmt.Errorf("Lyft mode %q is not supported", rawMode)
	}
	return nil
}

// Atlantis HTTPS SSL secrets
type SSLSecrets struct {
	CertFile string `type:"filecontent" help:"${help_ssl_cert_file}"`
	KeyFile  string `help:"${help_ssl_key_file}"`
}

func (s SSLSecrets) Validate() error {
	if (s.KeyFile == "") != (s.CertFile == "") {
		return fmt.Errorf("both ssl key and certificate are required")
	}
	return nil
}

// VCS Azure Devops secrets
type AzureDevopsSecrets struct {
	WebhookPassword string `help:"${help_ad_webhook_password}"`
	WebhookUser     string `help:"${help_ad_webhook_user}"`
	Token           string `help:"${help_ad_token}"` // nolint: gosec
	User            User   `help:"${help_ad_user}"`
}

func (s AzureDevopsSecrets) Validate() error {
	if (s.User == "") != (s.Token == "") {
		return fmt.Errorf("AzureDevops: both user and token should be set")
	}
	return nil
}

// VCS Bitbucket secrets
type BitbucketSecrets struct {
	BaseURL       HttpUrl `help:"${help_bitbucket_base_url}" default:"${default_bitbucket_base_url}"`
	Token         string  `help:"${help_bitbucket_token}"`
	User          User    `help:"${help_bitbucket_user}"`
	WebhookSecret string  `help:"${help_bitbucket_webhook_secret}"`
}

func (s BitbucketSecrets) Validate() error {
	if (s.User == "") != (s.Token == "") {
		return fmt.Errorf("Bitbucket: both user and token should be set")
	}
	if s.BaseURL.String() == bitbucketcloud.BaseURL && s.WebhookSecret != "" {
		return fmt.Errorf("Bitbucket: webhook secret for Bitbucket Cloud is not supported")
	}
	return nil
}

// VCS Github secrets
type GithubSecrets struct {
	Hostname      Schemeless `default:"github.com" help:"${help_gh_hostname}"`
	Token         string     `help:"${help_gh_token}"`
	User          User       `help:"${help_gh_user}"`
	AppID         int64      `help:"${help_gh_app_id}"`
	AppKey        string     `help:"${help_gh_app_key}"`
	AppKeyFile    string     `type:"filecontent" help:"${help_gh_app_key_file}"`
	AppSlug       string     `help:"${help_gh_app_slug}"`
	Org           string     `help:"${help_gh_organization}"`
	WebhookSecret string     `help:"${help_gh_webhook_secret}"` // nolint: gosec
}

func (s GithubSecrets) Validate() error {
	if (s.User == "") != (s.Token == "") {
		return fmt.Errorf("Github: both user and token should be set")
	}
	if (s.AppID == 0) != (s.AppKey == "" && s.AppKeyFile == "") {
		return fmt.Errorf("Github: either app key or app key file should be set together with app ID")
	}
	return nil
}

// VCS Gitlab secrets
type GitlabSecrets struct {
	Hostname      Schemeless `default:"gitlab.com" help:"${help_gitlab_hostname}"`
	Token         string     `help:"${help_gitlab_token}"`
	User          User       `help:"${help_gitlab_user}"`
	WebhookSecret string     `help:"${help_gitlab_webhook_secret}"` // nolint: gosec
}

func (s GitlabSecrets) Validate() error {
	if (s.User == "") != (s.Token == "") {
		return fmt.Errorf("Gitlab: both user and token should be set")
	}
	return nil
}

type UserConfig struct {
	Silence                   bool             `short:"s" help:"Silence Atlantis warnings"`
	AtlantisURL               HttpUrl          `help:"${help_atlantis_url}"`
	AutoplanFileList          Matcher          `default:"${default_autoplan_file_list}" help:"${help_autoplan_file_list}"`
	Config                    ConfigFlag       `help:"${help_config}"`
	CheckoutStrategy          string           `default:"branch" enum:"branch,merge" help:"${help_checkout_strategy}"`
	DataDir                   string           `type:"path" default:"~/.atlantis" help:"${help_data_dir}"`
	DefaultTFVersion          string           `help:"${help_default_tf_version}"`
	DisableApplyAll           bool             `help:"${help_disable_apply_all}"`
	DisableApply              bool             `help:"${help_disable_apply}"`
	DisableAutoplan           bool             `help:"${help_disable_autoplan}"`
	DisableMarkdownFolding    bool             `help:"${help_disable_markdown_folding}"`
	EnableRegexpCmd           bool             `help:"${help_enable_regexp_cmd}"`
	EnableDiffMarkdownFormat  bool             `help:"${help_enable_diff_markdown_format}"`
	EnablePolicyChecks        bool             `help:"${help_enable_policy_checks}"`
	FFOwner                   string           `help:"${help_ff_owner}"`
	FFRepo                    string           `help:"${help_ff_repo}"`
	FFBranch                  string           `help:"${help_ff_branch}"`
	FFPath                    string           `help:"${help_ff_path}"`
	HidePrevPlanComments      bool             `help:"${help_hide_prev_plan_comments}"`
	LogLevel                  logging.LogLevel `default:"info" enuam:"debug,info,warn,error" help:"${help_log_level}"`
	ParallelPoolSize          int              `default:"15" help:"${help_parallel_pool_size}"`
	MaxProjectsPerPR          int              `help:"${help_max_projects_per_pr}"`
	StatsNamespace            string           `default:"atlantis" help:"${help_stats_namespace}"`
	AllowDraftPrs             bool             `help:"${help_allow_draft_prs}"`
	Port                      int              `default:"4141" help:"${help_port}"`
	RepoConfig                string           `help:"${help_repo_config}"`
	RepoConfigJSON            string           `help:"${help_repo_config_json}"`
	RepoAllowlist             []Schemeless     `help:"${help_repo_allowlist}" required`
	SlackToken                string           `help:"${help_slack_token}"`
	TFDownloadURL             HttpUrl          `default:"https://releases.hashicorp.com" help:"${help_tf_download_url}"`
	VCSStatusName             string           `default:"atlantis" help:"${help_vcs_status_name}"`
	WriteGitCreds             bool             `help:"${help_write_git_creds}"`
	LyftAuditJobsSnsTopicArn  string           `help:"${help_lyft_audit_jobs_sns_topic_arn}"`
	LyftGatewaySnsTopicArn    string           `help:"${help_lyft_gateway_sns_topic_arn}"`
	LyftMode                  Mode             `enuam:"default,gateway,worker,hybrid" default:"default" help:"${help_lyft_mode}"`
	LyftWorkerQueueURL        HttpUrl          `help:"${help_lyft_worker_queue_url}"`
	DefaultCheckrunDetailsURL HttpUrl          `help:"${help_default_checkrun_details_url}"`
	AtlantisVersion           string           `kong:"-"`
	Webhooks                  []WebhookConfig  `kong:"-"`
	SSLSecrets                `kong:"embed,prefix='ssl-'"`
	AzureDevopsSecrets        `kong:"embed,prefix='azuredevops-'"`
	GithubSecrets             `kong:"embed,prefix='gh-'"`
	GitlabSecrets             `kong:"embed,prefix='gitlab-'"`
	BitbucketSecrets          `kong:"embed,prefix='bitbucket-'"`
}
