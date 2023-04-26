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
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/assert"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/runatlantis/atlantis/server"
	"github.com/runatlantis/atlantis/server/events/vcs/fixtures"
	"github.com/runatlantis/atlantis/server/logging"
	. "github.com/runatlantis/atlantis/testing"
	"gopkg.in/yaml.v3"
)

const atlantisVersion = "test-version"

// Adding a new flag? Add it to this slice for testing in alphabetical
// order.

type flagValue struct {
	Input  interface{}
	Output interface{}
}

var testFlags = map[string]flagValue{
	"azuredevops-token": {
		Input:  "ad-token",
		Output: "ad-token",
	},
	"azuredevops-user": {
		Input:  "ad-user",
		Output: server.User("ad-user"),
	},
	"azuredevops-webhook-password": {
		Input:  "ad-wh-pass",
		Output: "ad-wh-pass",
	},
	"azuredevops-webhook-user": {
		Input:  "ad-wh-user",
		Output: "ad-wh-user",
	},
	"atlantis-url": {
		Input: "http://url",
		Output: server.HttpUrl{
			&url.URL{
				Host:   "url",
				Scheme: "http",
			},
		},
	},
	"bitbucket-base-url": {
		Input: "https://bitbucket-base-url.com",
		Output: server.HttpUrl{
			&url.URL{
				Scheme: "https",
				Host:   "bitbucket-base-url.com",
			},
		},
	},
	"bitbucket-token": {
		Input:  "bitbucket-token",
		Output: "bitbucket-token",
	},
	"bitbucket-user": {
		Input:  "bitbucket-user",
		Output: server.User("bitbucket-user"),
	},
	"bitbucket-webhook-secret": {
		Input:  "bitbucket-secret",
		Output: "bitbucket-secret",
	},
	"checkout-strategy": {
		Input:  "merge",
		Output: "merge",
	},
	"data-dir": {
		Input:  "/path",
		Output: "/path",
	},
	"default-tf-version": {
		Input:  "v0.11.0",
		Output: "v0.11.0",
	},
	"disable-apply-all": {
		Input:  true,
		Output: true,
	},
	"disable-apply": {
		Input:  true,
		Output: true,
	},
	"disable-markdown-folding": {
		Input:  true,
		Output: true,
	},
	"gh-hostname": {
		Input: "ghhostname",
		Output: server.Schemeless{
			&url.URL{
				Host: "ghhostname",
			},
		},
	},
	"gh-token": {
		Input:  "token",
		Output: "token",
	},
	"gh-user": {
		Input:  "user",
		Output: server.User("user"),
	},
	"gh-app-slug": {
		Input:  "atlantis",
		Output: "atlantis",
	},
	"gh-webhook-secret": {
		Input:  "secret",
		Output: "secret",
	},
	"gitlab-hostname": {
		Input: "gitlab-hostname",
		Output: server.Schemeless{
			&url.URL{
				Scheme: "",
				Host:   "gitlab-hostname",
			},
		},
	},
	"gitlab-token": {
		Input:  "gitlab-token",
		Output: "gitlab-token",
	},
	"gitlab-user": {
		Input:  "gitlab-user",
		Output: server.User("gitlab-user"),
	},
	"gitlab-webhook-secret": {
		Input:  "gitlab-secret",
		Output: "gitlab-secret",
	},
	"log-level": {
		Input:  "debug",
		Output: logging.Debug,
	},
	"stats-namespace": {
		Input:  "atlantis",
		Output: "atlantis",
	},
	"allow-draft-prs": {
		Input:  true,
		Output: true,
	},
	"parallel-pool-size": {
		Input:  100,
		Output: 100,
	},
	"repo-allowlist": {
		Input: "github.com/runatlantis/atlantis",
		Output: []server.Schemeless{{
			&url.URL{
				Host: "github.com",
				Path: "/runatlantis/atlantis",
			},
		}},
	},
	"slack-token": {
		Input:  "slack-token",
		Output: "slack-token",
	},
	"ssl-cert-file": {
		Input:  "cert-file",
		Output: "cert-file",
	},
	"ssl-key-file": {
		Input:  "key-file",
		Output: "key-file",
	},
	"tf-download-url": {
		Input: "https://my-hostname.com",
		Output: server.HttpUrl{
			&url.URL{
				Scheme: "https",
				Host:   "my-hostname.com",
			},
		},
	},
	"vcs-status-name": {
		Input:  "my-status",
		Output: "my-status",
	},
	"write-git-creds": {
		Input:  true,
		Output: true,
	},
	"disable-autoplan": {
		Input:  true,
		Output: true,
	},
}

func TestRun_Defaults(t *testing.T) {
	t.Log("Should set the defaults for all unspecified flags.")

	_, err := setup(map[string]flagValue{
		"gh-user": {
			Input: "user",
		},
		"gh-token": {
			Input: "token",
		},
		"gh-hostname": {
			Input: "ghhostname",
		},
		"repo-allowlist": {
			Input: "*",
		},
	}, t)
	Ok(t, err)
	/*
	   *

	   	// Get our hostname since that's what atlantis-url gets defaulted to.
	   	hostname, err := os.Hostname()
	   	Ok(t, err)

	   	// Get our home dir since that's what data-dir defaulted to.
	   	dataDir, err := homedir.Expand("~/.atlantis")
	   	Ok(t, err)

	   	strExceptions := map[string]string{
	   		"gh-user":        "user",
	   		"gh-token":       "token",
	   		"data-dir":       dataDir,
	   		"atlantis_url":   "http://" + hostname + ":4141",
	   		"repo-allowlist": "*",
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
	*/
}

func TestRun_Flags(t *testing.T) {
	t.Log("Should use all flags that are set.")
	c, err := setup(testFlags, t)
	Ok(t, err)
	for flag, exp := range testFlags {
		Equals(t, exp.Output, configVal(t, c, flag))
	}
}

func TestRun_GHAppKeyFile(t *testing.T) {
	t.Log("Should use all the values from the config file.")
	tmpFile := tempFile(t, "testdata")
	defer os.Remove(tmpFile) // nolint: errcheck
	_, err := setup(map[string]flagValue{
		"gh-app-key-file": {
			Input: tmpFile,
		},
		"gh-app-id": {
			Input: "1",
		},
		"gh-hostname": {
			Input: "ghhostname",
		},
		"repo-allowlist": {
			Input: "*",
		},
	}, t)
	assert.NoError(t, err)
}

func TestRun_ConfigFile(t *testing.T) {
	t.Log("Should use all the values from the config file.")
	// Use yaml package to quote values that need quoting
	cfg := make(map[string]map[string]interface{})
	cfg["server"] = make(map[string]interface{})
	for flag, val := range testFlags {
		cfg["server"][flag] = val.Input
	}
	cfgContents, yamlErr := yaml.Marshal(&cfg)
	Ok(t, yamlErr)
	tmpFile := tempFile(t, string(cfgContents))
	defer os.Remove(tmpFile) // nolint: errcheck
	c, err := setup(map[string]flagValue{
		"config": {
			Input: tmpFile,
		},
	}, t)
	Ok(t, err)
	for flag, exp := range testFlags {
		Equals(t, exp.Output, configVal(t, c, flag))
	}
}

func TestRun_EnvironmentVariables(t *testing.T) {
	t.Log("Environment variables should work.")
	for flag, value := range testFlags {
		envKey := "ATLANTIS_" + strings.ToUpper(strings.ReplaceAll(flag, "-", "_"))
		envVal := ""
		switch value.Input.(type) {
		case string:
			envVal = value.Input.(string)
		case bool:
			envVal = fmt.Sprintf("%t", value.Input.(bool))
		case int:
			envVal = fmt.Sprintf("%d", value.Input.(int))
		}
		os.Setenv(envKey, envVal) // nolint: errcheck
		defer func(key string) { os.Unsetenv(key) }(envKey)
	}
	c, err := setup(nil, t)
	Ok(t, err)
	for flag, exp := range testFlags {
		Equals(t, exp.Output, configVal(t, c, flag))
	}
}

func TestRun_NoConfigFlag(t *testing.T) {
	t.Log("If there is no config flag specified Run should return nil.")
	_, err := setup(map[string]flagValue{
		"config": {
			Input: "",
		},
		"gh-user": {
			Input: "user",
		},
		"gh-token": {
			Input: "token",
		},
		"gh-hostname": {
			Input: "ghhostname",
		},
		"repo-allowlist": {
			Input: "*",
		},
	}, t)
	Ok(t, err)
}

func TestRun_ConfigFileExtension(t *testing.T) {
	t.Log("If the config file doesn't have an extension then error.")
	_, err := setup(map[string]flagValue{
		"gh-user": {
			Input: "user",
		},
		"gh-token": {
			Input: "token",
		},
		"config": {
			Input: "does-not-exist",
		},
	}, t)
	Equals(t, "no loader for config with extension \"\" found", err.Error())
}

func TestRun_ConfigFileMissing(t *testing.T) {
	t.Log("If the config file doesn't exist then error.")
	_, err := setup(map[string]flagValue{
		"gh-user": {
			Input: "user",
		},
		"gh-token": {
			Input: "token",
		},
		"gh-hostname": {
			Input: "ghhostname",
		},
		"config": {
			Input: "does-not-exist.yaml",
		},
	}, t)
	p, _ := os.Getwd()
	Equals(t, fmt.Sprintf("open %s/does-not-exist.yaml: no such file or directory", p), err.Error())
}

func TestRun_ConfigFileExists(t *testing.T) {
	t.Log("If the config file exists then there should be no error.")
	tmpFile := tempFile(t, "---")
	defer os.Remove(tmpFile) // nolint: errcheck
	_, err := setup(map[string]flagValue{
		"gh-user": {
			Input: "user",
		},
		"gh-token": {
			Input: "token",
		},
		"gh-hostname": {
			Input: "ghhostname",
		},
		"repo-allowlist": {
			Input: "*",
		},
		"config": {
			Input: tmpFile,
		},
	}, t)
	Ok(t, err)
}

func TestRun_InvalidConfig(t *testing.T) {
	t.Log("If the config file contains invalid yaml there should be an error.")
	tmpFile := tempFile(t, "invalidyaml")
	defer os.Remove(tmpFile) // nolint: errcheck
	_, err := setup(map[string]flagValue{
		"gh-user": {
			Input: "user",
		},
		"gh-token": {
			Input: "token",
		},
		"config": {
			Input: tmpFile,
		},
	}, t)
	Assert(t, strings.Contains(err.Error(), "unmarshal errors"), "should be an unmarshal error")
}

func TestRun_ValidateLogLevel(t *testing.T) {
	cases := []struct {
		description string
		flags       map[string]flagValue
		expectError bool
	}{
		{
			"log level is invalid",
			map[string]flagValue{
				"log-level": {
					Input: "invalid",
				},
			},
			true,
		},
		{
			"log level is valid uppercase",
			map[string]flagValue{
				"log-level": {
					Input: "DEBUG",
				},
			},
			false,
		},
	}
	for _, testCase := range cases {
		t.Log("Should validate log level when " + testCase.description)
		for k, v := range testCase.flags {
			testFlags[k] = v
		}
		_, err := setup(testFlags, t)
		if testCase.expectError {
			Assert(t, err != nil, "should be an error")
		} else {
			Ok(t, err)
		}
	}
}

func TestRun_ValidateCheckoutStrategy(t *testing.T) {
	_, err := setup(map[string]flagValue{
		"checkout-strategy": {
			Input: "invalid",
		},
	}, t)
	ErrEquals(t, "--checkout-strategy must be one of \"branch\",\"merge\" but got \"invalid\"", err)
}

func TestRun_ValidateSSLConfig(t *testing.T) {
	expErr := "server: both ssl key and certificate are required"
	cases := []struct {
		description string
		flags       map[string]flagValue
		expectError bool
	}{
		{
			"neither option set",
			make(map[string]flagValue),
			false,
		},
		{
			"just ssl-key-file set",
			map[string]flagValue{
				"ssl-key-file": {
					Input: "file",
				},
				"ssl-cert-file": {
					Input: "",
				},
			},
			true,
		},
		{
			"just ssl-cert-file set",
			map[string]flagValue{
				"ssl-cert-file": {
					Input: "flag",
				},
				"ssl-key-file": {
					Input: "",
				},
			},
			true,
		},
		{
			"both flags set",
			map[string]flagValue{
				"ssl-cert-file": {
					Input: "cert",
				},
				"ssl-key-file": {
					Input: "key",
				},
			},
			false,
		},
	}
	for _, testCase := range cases {
		t.Log("Should validate ssl config when " + testCase.description)
		for k, v := range testCase.flags {
			testFlags[k] = v
		}
		_, err := setup(testFlags, t)
		if testCase.expectError {
			Assert(t, err != nil, "should be an error")
			Equals(t, expErr, err.Error())
		} else {
			Ok(t, err)
		}
	}
}

func TestRun_ValidateVCSConfig(t *testing.T) {
	expErr := "server: credentials for at least one VCS provider should be defined"
	cases := []struct {
		description string
		flags       map[string]flagValue
		expectError bool
		customError string
	}{
		{
			"no config set",
			make(map[string]flagValue),
			true,
			"",
		},
		{
			"just github token set",
			map[string]flagValue{
				"gh-token": {
					Input: "token",
				},
			},
			true,
			"",
		},
		{
			"just gitlab token set",
			map[string]flagValue{
				"gitlab-token": {
					Input: "token",
				},
			},
			true,
			"",
		},
		{
			"just bitbucket token set",
			map[string]flagValue{
				"bitbucket-token": {
					Input: "token",
				},
			},
			true,
			"",
		},
		{
			"just azuredevops token set",
			map[string]flagValue{
				"azuredevops-token": {
					Input: "token",
				},
			},
			true,
			"",
		},
		{
			"just github user set",
			map[string]flagValue{
				"gh-user": {
					Input: "user",
				},
			},
			true,
			"server: Github: both user and token should be set",
		},
		{
			"just github app set",
			map[string]flagValue{
				"gh-app-id": {
					Input: "1",
				},
			},
			true,
			"server: Github: either app key or app key file should be set together with app ID",
		},
		{
			"just github app key file set",
			map[string]flagValue{
				"gh-app-key-file": {
					Input: "key.pem",
				},
			},
			true,
			"",
		},
		{
			"just github app key set",
			map[string]flagValue{
				"gh-app-key": {
					Input: fixtures.GithubPrivateKey,
				},
			},
			true,
			"",
		},
		{
			"just gitlab user set",
			map[string]flagValue{
				"gitlab-user": {
					Input: "user",
				},
			},
			true,
			"server: Gitlab: both user and token should be set",
		},
		{
			"just bitbucket user set",
			map[string]flagValue{
				"bitbucket-user": {
					Input: "user",
				},
			},
			true,
			"server: Bitbucket: both user and token should be set",
		},
		{
			"just azuredevops user set",
			map[string]flagValue{
				"azuredevops-user": {
					Input: "user",
				},
			},
			true,
			"server: AzureDevops: both user and token should be set",
		},
		{
			"github user and gitlab token set",
			map[string]flagValue{
				"gh-user": {
					Input: "user",
				},
				"gitlab-token": {
					Input: "token",
				},
			},
			true,
			"server: Github: both user and token should be set",
		},
		{
			"gitlab user and github token set",
			map[string]flagValue{
				"gitlab-user": {
					Input: "user",
				},
				"gh-token": {
					Input: "token",
				},
			},
			true,
			"server: Github: both user and token should be set",
		},
		{
			"github user and bitbucket token set",
			map[string]flagValue{
				"gh-user": {
					Input: "user",
				},
				"bitbucket-token": {
					Input: "token",
				},
			},
			true,
			"server: Bitbucket: both user and token should be set",
		},
		{
			"github user and github token set and should be successful",
			map[string]flagValue{
				"gh-user": {
					Input: "user",
				},
				"gh-token": {
					Input: "token",
				},
			},
			false,
			"",
		},
		{
			"github app and key set and should be successful",
			map[string]flagValue{
				"gh-app-id": {
					Input: "1",
				},
				"gh-app-key": {
					Input: fixtures.GithubPrivateKey,
				},
			},
			false,
			"",
		},
		{
			"gitlab user and gitlab token set and should be successful",
			map[string]flagValue{
				"gitlab-user": {
					Input: "user",
				},
				"gitlab-token": {
					Input: "token",
				},
			},
			false,
			"",
		},
		{
			"bitbucket user and bitbucket token set and should be successful",
			map[string]flagValue{
				"bitbucket-user": {
					Input: "user",
				},
				"bitbucket-token": {
					Input: "token",
				},
			},
			false,
			"",
		},
		{
			"azuredevops user and azuredevops token set and should be successful",
			map[string]flagValue{
				"azuredevops-user": {
					Input: "user",
				},
				"azuredevops-token": {
					Input: "token",
				},
			},
			false,
			"",
		},
		{
			"all set should be successful",
			map[string]flagValue{
				"gh-user": {
					Input: "user",
				},
				"gh-token": {
					Input: "token",
				},
				"gitlab-user": {
					Input: "user",
				},
				"gitlab-token": {
					Input: "token",
				},
				"bitbucket-user": {
					Input: "user",
				},
				"bitbucket-token": {
					Input: "token",
				},
				"azuredevops-user": {
					Input: "user",
				},
				"azuredevops-token": {
					Input: "token",
				},
			},
			false,
			"",
		},
	}
	for _, testCase := range cases {
		t.Log("Should validate vcs config when " + testCase.description)
		testCase.flags["repo-allowlist"] = flagValue{
			Input: "*",
		}

		_, err := setup(testCase.flags, t)
		if testCase.expectError {
			Assert(t, err != nil, "should be an error")
			testErr := expErr
			if testCase.customError != "" {
				testErr = testCase.customError
			}
			Equals(t, testErr, err.Error())
		} else {
			Ok(t, err)
		}
	}
}

func TestRun_ExpandHomeInDataDir(t *testing.T) {
	t.Log("If ~ is used as a data-dir path, should expand to absolute home path")
	c, err := setup(map[string]flagValue{
		"gh-user": {
			Input: "user",
		},
		"gh-token": {
			Input: "token",
		},
		"repo-allowlist": {
			Input: "*",
		},
		"data-dir": {
			Input: "~/this/is/a/path",
		},
	}, t)
	Ok(t, err)
	serverCmd := c.Selected().Target.Interface().(ServerCmd)
	home, err := homedir.Dir()
	Ok(t, err)
	Equals(t, home+"/this/is/a/path", serverCmd.UserConfig.DataDir)
}

func TestRun_RelativeDataDir(t *testing.T) {
	t.Log("Should convert relative dir to absolute.")
	// Figure out what ../ should be as an absolute path.
	expectedAbsolutePath, err := filepath.Abs("../")
	Ok(t, err)
	testFlags["data-dir"] = flagValue{
		Input: "../",
	}
	c, err := setup(testFlags, t)

	serverCmd := c.Selected().Target.Interface().(ServerCmd)
	Equals(t, expectedAbsolutePath, serverCmd.UserConfig.DataDir)
}

func TestRun_GithubUser(t *testing.T) {
	t.Log("Should remove the @ from the github username if it's passed.")
	c, err := setup(map[string]flagValue{
		"gh-user": {
			Input: "@user",
		},
		"gh-token": {
			Input: "token",
		},
		"repo-allowlist": {
			Input: "*",
		},
	}, t)
	Ok(t, err)
	serverCmd := c.Selected().Target.Interface().(ServerCmd)
	Equals(t, server.User("user"), serverCmd.UserConfig.GithubSecrets.User)
}

func TestRun_GithubApp(t *testing.T) {
	t.Log("Should remove the @ from the github username if it's passed.")
	c, err := setup(map[string]flagValue{
		"gh-app-key": {
			Input: fixtures.GithubPrivateKey,
		},
		"gh-app-id": {
			Input: "1",
		},
		"repo-allowlist": {
			Input: "*",
		},
	}, t)
	Ok(t, err)
	serverCmd := c.Selected().Target.Interface().(ServerCmd)
	Equals(t, int64(1), serverCmd.UserConfig.GithubSecrets.AppID)
}

func TestRun_GitlabUser(t *testing.T) {
	t.Log("Should remove the @ from the gitlab username if it's passed.")
	c, err := setup(map[string]flagValue{
		"gitlab-user": {
			Input: "@user",
		},
		"gitlab-token": {
			Input: "token",
		},
		"repo-allowlist": {
			Input: "*",
		},
	}, t)
	Ok(t, err)
	serverCmd := c.Selected().Target.Interface().(ServerCmd)
	Equals(t, server.User("user"), serverCmd.UserConfig.GitlabSecrets.User)
}

func TestRun_BitbucketUser(t *testing.T) {
	t.Log("Should remove the @ from the bitbucket username if it's passed.")
	c, err := setup(map[string]flagValue{
		"bitbucket-user": {
			Input: "@user",
		},
		"bitbucket-token": {
			Input: "token",
		},
		"repo-allowlist": {
			Input: "*",
		},
	}, t)
	Ok(t, err)
	serverCmd := c.Selected().Target.Interface().(ServerCmd)
	Equals(t, server.User("user"), serverCmd.UserConfig.BitbucketSecrets.User)
}

func TestRun_ADUser(t *testing.T) {
	t.Log("Should remove the @ from the azure devops username if it's passed.")
	c, err := setup(map[string]flagValue{
		"azuredevops-user": {
			Input: "@user",
		},
		"azuredevops-token": {
			Input: "token",
		},
		"repo-allowlist": {
			Input: "*",
		},
	}, t)
	Ok(t, err)
	serverCmd := c.Selected().Target.Interface().(ServerCmd)
	Equals(t, server.User("user"), serverCmd.UserConfig.AzureDevopsSecrets.User)
}

// If using bitbucket cloud, webhook secrets are not supported.
func TestRun_BitbucketCloudWithWebhookSecret(t *testing.T) {
	_, err := setup(map[string]flagValue{
		"bitbucket-user": {
			Input: "user",
		},
		"bitbucket-token": {
			Input: "token",
		},
		"repo-allowlist": {
			Input: "*",
		},
		"bitbucket-webhook-secret": {
			Input: "my secret",
		},
	}, t)
	ErrEquals(t, "server: Bitbucket: webhook secret for Bitbucket Cloud is not supported", err)
}

// Base URL must have a scheme.
func TestRun_BitbucketServerBaseURLScheme(t *testing.T) {
	_, err := setup(map[string]flagValue{
		"bitbucket-user": {
			Input: "user",
		},
		"bitbucket-token": {
			Input: "token",
		},
		"repo-allowlist": {
			Input: "*",
		},
		"bitbucket-base-url": {
			Input: "mydomain.com",
		},
	}, t)
	ErrEquals(t, "--bitbucket-base-url: failed to parse HTTP url: protocol \"\" is not supported", err)

	_, err = setup(map[string]flagValue{
		"bitbucket-user": {
			Input: "user",
		},
		"bitbucket-token": {
			Input: "token",
		},
		"repo-allowlist": {
			Input: "*",
		},
		"bitbucket-base-url": {
			Input: "://mydomain.com",
		},
	}, t)
	ErrEquals(t, "--bitbucket-base-url: parse \"://mydomain.com\": missing protocol scheme", err)
}

// Port should be retained on base url.
func TestRun_BitbucketServerBaseURLPort(t *testing.T) {
	c, err := setup(map[string]flagValue{
		"bitbucket-user": {
			Input: "user",
		},
		"bitbucket-token": {
			Input: "token",
		},
		"repo-allowlist": {
			Input: "*",
		},
		"bitbucket-base-url": {
			Input: "http://mydomain.com:7990",
		},
	}, t)
	Ok(t, err)
	serverCmd := c.Selected().Target.Interface().(ServerCmd)
	Equals(t, "http://mydomain.com:7990", serverCmd.BitbucketSecrets.BaseURL.String())
}

// Can't use both --repo-config and --repo-config-json.
func TestRun_RepoCfgFlags(t *testing.T) {
	_, err := setup(map[string]flagValue{
		"gh-user": {
			Input: "user",
		},
		"gh-token": {
			Input: "token",
		},
		"repo-allowlist": {
			Input: "github.com",
		},
		"repo-config": {
			Input: "repos.yaml",
		},
		"repo-config-json": {
			Input: "{}",
		},
	}, t)
	ErrEquals(t, "server: cannot set both path to repo config and repo config json at the same time", err)
}

// Must set allow or whitelist.
func TestRun_Allowlist(t *testing.T) {
	_, err := setup(map[string]flagValue{
		"gh-user": {
			Input: "user",
		},
		"gh-token": {
			Input: "token",
		},
	}, t)
	ErrEquals(t, "missing flags: --repo-allowlist=REPO-ALLOWLIST,...", err)
}

func TestRun_AutoplanFileList(t *testing.T) {
	cases := []struct {
		description string
		flags       map[string]flagValue
		expectErr   string
	}{
		{
			"valid value",
			map[string]flagValue{
				"autoplan-file-list": {
					Input: "**/*.tf",
				},
			},
			"",
		},
		{
			"invalid exclusion pattern",
			map[string]flagValue{
				"autoplan-file-list": {
					Input: "**/*.yml,!",
				},
			},
			"--autoplan-file-list: illegal exclusion pattern: \"!\"",
		},
		{
			"invalid pattern",
			map[string]flagValue{
				"autoplan-file-list": {
					Input: "[^]",
				},
			},
			"--autoplan-file-list: syntax error in pattern",
		},
	}
	for _, testCase := range cases {
		t.Log("Should validate autoplan file list when " + testCase.description)
		for k, v := range testCase.flags {
			testFlags[k] = v
		}
		_, err := setup(testFlags, t)
		if testCase.expectErr != "" {
			ErrEquals(t, testCase.expectErr, err)
		} else {
			Ok(t, err)
		}
	}
}

func setup(args map[string]flagValue, _ *testing.T) (*kong.Context, error) {
	parser, _ := kong.New(
		&CLI,
		FlagsVars,
		kong.DefaultEnvars("ATLANTIS"),
	)

	cmdline := []string{"server"}
	for k, v := range args {
		val := ""
		switch v.Input.(type) {
		case bool:
			val = fmt.Sprintf("%t", v.Input.(bool))
		case string:
			val = v.Input.(string)
		case int:
			val = fmt.Sprintf("%d", v.Input.(int))
		}
		cmdline = append(cmdline, fmt.Sprintf("--%s=%s", k, val))
	}
	return parser.Parse(cmdline)
}

func tempFile(t *testing.T, contents string) string {
	f, err := os.CreateTemp("", "")
	Ok(t, err)
	newName := f.Name() + ".yaml"
	err = os.Rename(f.Name(), newName)
	Ok(t, err)
	os.WriteFile(newName, []byte(contents), 0600) // nolint: errcheck
	return newName
}

func configVal(t *testing.T, ctx *kong.Context, tag string) interface{} {
	t.Helper()
	for _, flag := range ctx.Flags() {
		if flag.Name == tag {
			return ctx.FlagValue(flag)
		}
	}
	t.Fatalf("no field with tag %q found", tag)
	return ""
}
