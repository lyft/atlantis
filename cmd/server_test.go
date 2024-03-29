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
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	homedir "github.com/mitchellh/go-homedir"
	server "github.com/runatlantis/atlantis/server/legacy"
	"github.com/runatlantis/atlantis/server/legacy/events/vcs/fixtures"
	. "github.com/runatlantis/atlantis/testing"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

// passedConfig is set to whatever config ended up being passed to NewServer.
// Used for testing.
var passedConfig server.UserConfig

type ServerCreatorMock struct{}

func (s *ServerCreatorMock) NewServer(userConfig server.UserConfig, config server.Config) (ServerStarter, error) {
	passedConfig = userConfig
	return &ServerStarterMock{}, nil
}

type ServerStarterMock struct{}

func (s *ServerStarterMock) Start() error {
	return nil
}

// Adding a new flag? Add it to this slice for testing in alphabetical
// order.
var testFlags = map[string]interface{}{
	AtlantisURLFlag:              "url",
	AutoplanFileListFlag:         "**/*.tf,**/*.yml",
	CheckoutStrategyFlag:         "merge",
	DataDirFlag:                  "/path",
	DefaultTFVersionFlag:         "v0.11.0",
	DisableApplyAllFlag:          true,
	DisableApplyFlag:             true,
	DisableMarkdownFoldingFlag:   true,
	GHHostnameFlag:               "ghhostname",
	GHTokenFlag:                  "token",
	GHUserFlag:                   "user",
	GHAppIDFlag:                  int64(0),
	GHAppKeyFileFlag:             "",
	GHAppSlugFlag:                "atlantis",
	GHOrganizationFlag:           "",
	GHWebhookSecretFlag:          "secret",
	LogLevelFlag:                 "debug",
	StatsNamespace:               "atlantis",
	AllowDraftPRs:                true,
	PortFlag:                     8181,
	ParallelPoolSize:             100,
	RepoAllowlistFlag:            "github.com/runatlantis/atlantis",
	SlackTokenFlag:               "slack-token",
	SSLCertFileFlag:              "cert-file",
	SSLKeyFileFlag:               "key-file",
	TFDownloadURLFlag:            "https://my-hostname.com",
	VCSStatusName:                "my-status",
	WriteGitFileFlag:             true,
	LyftAuditJobsSnsTopicArnFlag: "",
	LyftGatewaySnsTopicArnFlag:   "",
	LyftModeFlag:                 "",
	LyftWorkerQueueURLFlag:       "",
	DisableAutoplanFlag:          true,
	EnableRegExpCmdFlag:          false,
	EnableDiffMarkdownFormat:     false,
}

func TestExecute_Defaults(t *testing.T) {
	t.Log("Should set the defaults for all unspecified flags.")

	c := setup(map[string]interface{}{
		GHUserFlag:        "user",
		GHTokenFlag:       "token",
		RepoAllowlistFlag: "*",
	}, t)
	err := c.Execute()
	Ok(t, err)

	// Get our hostname since that's what atlantis-url gets defaulted to.
	hostname, err := os.Hostname()
	Ok(t, err)

	// Get our home dir since that's what data-dir defaulted to.
	dataDir, err := homedir.Expand("~/.atlantis")
	Ok(t, err)

	strExceptions := map[string]string{
		GHUserFlag:        "user",
		GHTokenFlag:       "token",
		DataDirFlag:       dataDir,
		AtlantisURLFlag:   "http://" + hostname + ":4141",
		RepoAllowlistFlag: "*",
	}
	strIgnore := map[string]bool{
		"config": true,
	}
	for flag, cfg := range stringFlags {
		t.Log(flag)
		if _, ok := strIgnore[flag]; ok {
			continue
		} else if excep, ok := strExceptions[flag]; ok {
			Equals(t, excep, configVal(t, passedConfig, flag))
		} else {
			Equals(t, cfg.defaultValue, configVal(t, passedConfig, flag))
		}
	}
	for flag, cfg := range boolFlags {
		t.Log(flag)
		Equals(t, cfg.defaultValue, configVal(t, passedConfig, flag))
	}
	for flag, cfg := range intFlags {
		t.Log(flag)
		Equals(t, cfg.defaultValue, configVal(t, passedConfig, flag))
	}
}

func TestExecute_Flags(t *testing.T) {
	t.Log("Should use all flags that are set.")
	c := setup(testFlags, t)
	err := c.Execute()
	Ok(t, err)
	for flag, exp := range testFlags {
		Equals(t, exp, configVal(t, passedConfig, flag))
	}
}

func TestExecute_GHAppKeyFile(t *testing.T) {
	t.Log("Should use all the values from the config file.")
	tmpFile := tempFile(t, "testdata")
	defer os.Remove(tmpFile) // nolint: errcheck
	c := setup(map[string]interface{}{
		GHAppKeyFileFlag:  tmpFile,
		GHAppIDFlag:       int64(1),
		RepoAllowlistFlag: "*",
	}, t)
	err := c.Execute()
	assert.NoError(t, err)
}

func TestExecute_ConfigFile(t *testing.T) {
	t.Log("Should use all the values from the config file.")
	// Use yaml package to quote values that need quoting
	cfgContents, yamlErr := yaml.Marshal(&testFlags)
	Ok(t, yamlErr)
	tmpFile := tempFile(t, string(cfgContents))
	defer os.Remove(tmpFile) // nolint: errcheck
	c := setup(map[string]interface{}{
		ConfigFlag: tmpFile,
	}, t)
	err := c.Execute()
	Ok(t, err)
	for flag, exp := range testFlags {
		Equals(t, exp, configVal(t, passedConfig, flag))
	}
}

func TestExecute_EnvironmentVariables(t *testing.T) {
	t.Log("Environment variables should work.")
	for flag, value := range testFlags {
		envKey := "ATLANTIS_" + strings.ToUpper(strings.ReplaceAll(flag, "-", "_"))
		os.Setenv(envKey, fmt.Sprintf("%v", value)) // nolint: errcheck
		defer func(key string) { os.Unsetenv(key) }(envKey)
	}
	c := setup(nil, t)
	err := c.Execute()
	Ok(t, err)
	for flag, exp := range testFlags {
		Equals(t, exp, configVal(t, passedConfig, flag))
	}
}

func TestExecute_NoConfigFlag(t *testing.T) {
	t.Log("If there is no config flag specified Execute should return nil.")
	c := setupWithDefaults(map[string]interface{}{
		ConfigFlag: "",
	}, t)
	err := c.Execute()
	Ok(t, err)
}

func TestExecute_ConfigFileExtension(t *testing.T) {
	t.Log("If the config file doesn't have an extension then error.")
	c := setupWithDefaults(map[string]interface{}{
		ConfigFlag: "does-not-exist",
	}, t)
	err := c.Execute()
	Equals(t, "invalid config: reading does-not-exist: Unsupported Config Type \"\"", err.Error())
}

func TestExecute_ConfigFileMissing(t *testing.T) {
	t.Log("If the config file doesn't exist then error.")
	c := setupWithDefaults(map[string]interface{}{
		ConfigFlag: "does-not-exist.yaml",
	}, t)
	err := c.Execute()
	Equals(t, "invalid config: reading does-not-exist.yaml: open does-not-exist.yaml: no such file or directory", err.Error())
}

func TestExecute_ConfigFileExists(t *testing.T) {
	t.Log("If the config file exists then there should be no error.")
	tmpFile := tempFile(t, "")
	defer os.Remove(tmpFile) // nolint: errcheck
	c := setupWithDefaults(map[string]interface{}{
		ConfigFlag: tmpFile,
	}, t)
	err := c.Execute()
	Ok(t, err)
}

func TestExecute_InvalidConfig(t *testing.T) {
	t.Log("If the config file contains invalid yaml there should be an error.")
	tmpFile := tempFile(t, "invalidyaml")
	defer os.Remove(tmpFile) // nolint: errcheck
	c := setupWithDefaults(map[string]interface{}{
		ConfigFlag: tmpFile,
	}, t)
	err := c.Execute()
	Assert(t, strings.Contains(err.Error(), "unmarshal errors"), "should be an unmarshal error")
}

// Should error if the repo allowlist contained a scheme.
func TestExecute_RepoAllowlistScheme(t *testing.T) {
	c := setup(map[string]interface{}{
		GHUserFlag:        "user",
		GHTokenFlag:       "token",
		RepoAllowlistFlag: "http://github.com/*",
	}, t)
	err := c.Execute()
	Assert(t, err != nil, "should be an error")
	Equals(t, "--repo-allowlist cannot contain ://, should be hostnames only", err.Error())
}

func TestExecute_ValidateLogLevel(t *testing.T) {
	cases := []struct {
		description string
		flags       map[string]interface{}
		expectError bool
	}{
		{
			"log level is invalid",
			map[string]interface{}{
				LogLevelFlag: "invalid",
			},
			true,
		},
		{
			"log level is valid uppercase",
			map[string]interface{}{
				LogLevelFlag: "DEBUG",
			},
			false,
		},
	}
	for _, testCase := range cases {
		t.Log("Should validate log level when " + testCase.description)
		c := setupWithDefaults(testCase.flags, t)
		err := c.Execute()
		if testCase.expectError {
			Assert(t, err != nil, "should be an error")
		} else {
			Ok(t, err)
		}
	}
}

func TestExecute_ValidateCheckoutStrategy(t *testing.T) {
	c := setupWithDefaults(map[string]interface{}{
		CheckoutStrategyFlag: "invalid",
	}, t)
	err := c.Execute()
	ErrEquals(t, "invalid checkout strategy: not one of branch or merge", err)
}

func TestExecute_ValidateSSLConfig(t *testing.T) {
	expErr := "--ssl-key-file and --ssl-cert-file are both required for ssl"
	cases := []struct {
		description string
		flags       map[string]interface{}
		expectError bool
	}{
		{
			"neither option set",
			make(map[string]interface{}),
			false,
		},
		{
			"just ssl-key-file set",
			map[string]interface{}{
				SSLKeyFileFlag: "file",
			},
			true,
		},
		{
			"just ssl-cert-file set",
			map[string]interface{}{
				SSLCertFileFlag: "flag",
			},
			true,
		},
		{
			"both flags set",
			map[string]interface{}{
				SSLCertFileFlag: "cert",
				SSLKeyFileFlag:  "key",
			},
			false,
		},
	}
	for _, testCase := range cases {
		t.Log("Should validate ssl config when " + testCase.description)
		c := setupWithDefaults(testCase.flags, t)
		err := c.Execute()
		if testCase.expectError {
			Assert(t, err != nil, "should be an error")
			Equals(t, expErr, err.Error())
		} else {
			Ok(t, err)
		}
	}
}

func TestExecute_ValidateVCSConfig(t *testing.T) {
	expErr := "--gh-user/--gh-token or --gh-app-id/--gh-app-key-file or --gh-app-id/--gh-app-key must be set"
	cases := []struct {
		description string
		flags       map[string]interface{}
		expectError bool
	}{
		{
			"no config set",
			make(map[string]interface{}),
			true,
		},
		{
			"just github token set",
			map[string]interface{}{
				GHTokenFlag: "token",
			},
			true,
		},
		{
			"just github user set",
			map[string]interface{}{
				GHUserFlag: "user",
			},
			true,
		},
		{
			"just github app set",
			map[string]interface{}{
				GHAppIDFlag: "1",
			},
			true,
		},
		{
			"just github app key file set",
			map[string]interface{}{
				GHAppKeyFileFlag: "key.pem",
			},
			true,
		},
		{
			"just github app key set",
			map[string]interface{}{
				GHAppKeyFlag: fixtures.GithubPrivateKey,
			},
			true,
		},
		{
			"github user and github token set and should be successful",
			map[string]interface{}{
				GHUserFlag:  "user",
				GHTokenFlag: "token",
			},
			false,
		},
		{
			"github app and key set and should be successful",
			map[string]interface{}{
				GHAppIDFlag:  "1",
				GHAppKeyFlag: fixtures.GithubPrivateKey,
			},
			false,
		},
	}
	for _, testCase := range cases {
		t.Log("Should validate vcs config when " + testCase.description)
		testCase.flags[RepoAllowlistFlag] = "*"

		c := setup(testCase.flags, t)
		err := c.Execute()
		if testCase.expectError {
			Assert(t, err != nil, "should be an error")
			Equals(t, expErr, err.Error())
		} else {
			Ok(t, err)
		}
	}
}

func TestExecute_ExpandHomeInDataDir(t *testing.T) {
	t.Log("If ~ is used as a data-dir path, should expand to absolute home path")
	c := setup(map[string]interface{}{
		GHUserFlag:        "user",
		GHTokenFlag:       "token",
		RepoAllowlistFlag: "*",
		DataDirFlag:       "~/this/is/a/path",
	}, t)
	err := c.Execute()
	Ok(t, err)

	home, err := homedir.Dir()
	Ok(t, err)
	Equals(t, home+"/this/is/a/path", passedConfig.DataDir)
}

func TestExecute_RelativeDataDir(t *testing.T) {
	t.Log("Should convert relative dir to absolute.")
	c := setupWithDefaults(map[string]interface{}{
		DataDirFlag: "../",
	}, t)

	// Figure out what ../ should be as an absolute path.
	expectedAbsolutePath, err := filepath.Abs("../")
	Ok(t, err)

	err = c.Execute()
	Ok(t, err)
	Equals(t, expectedAbsolutePath, passedConfig.DataDir)
}

func TestExecute_GithubUser(t *testing.T) {
	t.Log("Should remove the @ from the github username if it's passed.")
	c := setup(map[string]interface{}{
		GHUserFlag:        "@user",
		GHTokenFlag:       "token",
		RepoAllowlistFlag: "*",
	}, t)
	err := c.Execute()
	Ok(t, err)

	Equals(t, "user", passedConfig.GithubUser)
}

func TestExecute_GithubApp(t *testing.T) {
	t.Log("Should remove the @ from the github username if it's passed.")
	c := setup(map[string]interface{}{
		GHAppKeyFlag:      fixtures.GithubPrivateKey,
		GHAppIDFlag:       "1",
		RepoAllowlistFlag: "*",
	}, t)
	err := c.Execute()
	Ok(t, err)

	Equals(t, int64(1), passedConfig.GithubAppID)
}

// Can't use both --repo-config and --repo-config-json.
func TestExecute_RepoCfgFlags(t *testing.T) {
	c := setup(map[string]interface{}{
		GHUserFlag:         "user",
		GHTokenFlag:        "token",
		RepoAllowlistFlag:  "github.com",
		RepoConfigFlag:     "repos.yaml",
		RepoConfigJSONFlag: "{}",
	}, t)
	err := c.Execute()
	ErrEquals(t, "cannot use --repo-config and --repo-config-json at the same time", err)
}

// Can't use both --repo-allowlist and --repo-whitelist
func TestExecute_BothAllowAndWhitelist(t *testing.T) {
	c := setup(map[string]interface{}{
		GHUserFlag:        "user",
		GHTokenFlag:       "token",
		RepoAllowlistFlag: "github.com",
		RepoWhitelistFlag: "github.com",
	}, t)
	err := c.Execute()
	ErrEquals(t, "both --repo-allowlist and --repo-whitelist cannot be set–use --repo-allowlist", err)
}

// Must set allow or whitelist.
func TestExecute_AllowAndWhitelist(t *testing.T) {
	c := setup(map[string]interface{}{
		GHUserFlag:  "user",
		GHTokenFlag: "token",
	}, t)
	err := c.Execute()
	ErrEquals(t, "--repo-allowlist must be set for security purposes", err)
}

// Test that we set the corresponding allow list values on the userConfig
// struct if the deprecated whitelist flags are used.
func TestExecute_RepoWhitelistDeprecation(t *testing.T) {
	c := setup(map[string]interface{}{
		GHUserFlag:        "user",
		GHTokenFlag:       "token",
		RepoWhitelistFlag: "*",
	}, t)
	err := c.Execute()
	Ok(t, err)
	Equals(t, "*", passedConfig.RepoAllowlist)
}

func TestExecute_AutoplanFileList(t *testing.T) {
	cases := []struct {
		description string
		flags       map[string]interface{}
		expectErr   string
	}{
		{
			"default value",
			map[string]interface{}{
				AutoplanFileListFlag: DefaultAutoplanFileList,
			},
			"",
		},
		{
			"valid value",
			map[string]interface{}{
				AutoplanFileListFlag: "**/*.tf",
			},
			"",
		},
		{
			"invalid exclusion pattern",
			map[string]interface{}{
				AutoplanFileListFlag: "**/*.yml,!",
			},
			"invalid pattern in --autoplan-file-list, **/*.yml,!: illegal exclusion pattern: \"!\"",
		},
		{
			"invalid pattern",
			map[string]interface{}{
				AutoplanFileListFlag: "[^]",
			},
			"invalid pattern in --autoplan-file-list, [^]: syntax error in pattern",
		},
	}
	for _, testCase := range cases {
		t.Log("Should validate autoplan file list when " + testCase.description)
		c := setupWithDefaults(testCase.flags, t)
		err := c.Execute()
		if testCase.expectErr != "" {
			ErrEquals(t, testCase.expectErr, err)
		} else {
			Ok(t, err)
		}
	}
}

func setup(flags map[string]interface{}, _ *testing.T) *cobra.Command {
	vipr := viper.New()
	for k, v := range flags {
		vipr.Set(k, v)
	}
	c := &ServerCmd{
		ServerCreator: &ServerCreatorMock{},
		Viper:         vipr,
		SilenceOutput: true,
	}
	return c.Init()
}

func setupWithDefaults(flags map[string]interface{}, _ *testing.T) *cobra.Command {
	vipr := viper.New()
	flags[GHUserFlag] = "user"
	flags[GHTokenFlag] = "token"
	flags[RepoAllowlistFlag] = "*"

	for k, v := range flags {
		vipr.Set(k, v)
	}
	c := &ServerCmd{
		ServerCreator: &ServerCreatorMock{},
		Viper:         vipr,
		SilenceOutput: true,
	}
	return c.Init()
}

func tempFile(t *testing.T, contents string) string {
	f, err := os.CreateTemp("", "")
	Ok(t, err)
	newName := f.Name() + ".yaml"
	err = os.Rename(f.Name(), newName)
	Ok(t, err)
	os.WriteFile(newName, []byte(contents), 0o600) // nolint: errcheck
	return newName
}

func configVal(t *testing.T, u server.UserConfig, tag string) interface{} {
	t.Helper()
	v := reflect.ValueOf(u)
	typeOfS := v.Type()
	for i := 0; i < v.NumField(); i++ {
		if typeOfS.Field(i).Tag.Get("mapstructure") == tag {
			return v.Field(i).Interface()
		}
	}
	t.Fatalf("no field with tag %q found", tag)
	return nil
}
