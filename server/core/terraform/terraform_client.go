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
//
// Package terraform handles the actual running of terraform commands.
package terraform

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/hashicorp/go-getter"
	"github.com/hashicorp/go-version"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"

	"github.com/runatlantis/atlantis/server/core/runtime/cache"
	runtime_models "github.com/runatlantis/atlantis/server/core/runtime/models"
	"github.com/runatlantis/atlantis/server/core/terraform/cloud"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/terraform/ansi"
	"github.com/runatlantis/atlantis/server/feature"
	"github.com/runatlantis/atlantis/server/handlers"
	"github.com/runatlantis/atlantis/server/logging"
)

var LogStreamingValidCmds = [...]string{"init", "plan", "apply"}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_terraform_client.go Client

type Client interface {
	// RunCommandWithVersion executes terraform with args in path. If v is nil,
	// it will use the default Terraform version. workspace is the Terraform
	// workspace which should be set as an environment variable.
	RunCommandWithVersion(ctx models.ProjectCommandContext, path string, args []string, envs map[string]string, v *version.Version, workspace string) (string, error)

	// EnsureVersion makes sure that terraform version `v` is available to use
	EnsureVersion(log logging.SimpleLogging, v *version.Version) error
}

type DefaultClient struct {
	// defaultVersion is the default version of terraform to use if another
	// version isn't specified.
	defaultVersion *version.Version
	binDir         string
	// downloader downloads terraform versions.
	downloader      Downloader
	downloadBaseURL string

	versionCache   cache.ExecutionVersionCache
	commandBuilder commandBuilder

	featureAllocator feature.Allocator
	*AsyncClient
}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_downloader.go Downloader

// Downloader is for downloading terraform versions.
type Downloader interface {
	GetFile(dst, src string, opts ...getter.ClientOption) error
	GetAny(dst, src string, opts ...getter.ClientOption) error
}

// versionRegex extracts the version from `terraform version` output.
//     Terraform v0.12.0-alpha4 (2c36829d3265661d8edbd5014de8090ea7e2a076)
//	   => 0.12.0-alpha4
//
//     Terraform v0.11.10
//	   => 0.11.10
var versionRegex = regexp.MustCompile("Terraform v(.*?)(\\s.*)?\n")

// NewClientWithDefaultVersion creates a new terraform client and pre-fetches the default version
func NewClientWithDefaultVersion(
	log logging.SimpleLogging,
	binDir string,
	cacheDir string,
	tfeToken string,
	tfeHostname string,
	defaultVersionStr string,
	defaultVersionFlagName string,
	tfDownloadURL string,
	tfDownloader Downloader,
	usePluginCache bool,
	fetchAsync bool,
	projectCmdOutputHandler handlers.ProjectCommandOutputHandler,
	featureAllocator feature.Allocator,
) (*DefaultClient, error) {
	loader := versionLoader{
		downloader:  tfDownloader,
		downloadURL: tfDownloadURL,
	}

	versionCache := cache.NewExecutionVersionLayeredLoadingCache(
		"terraform",
		binDir,
		loader.loadVersion,
	)

	version, err := getDefaultVersion(defaultVersionStr, defaultVersionFlagName)

	if err != nil {
		return nil, errors.Wrapf(err, "getting default version")
	}

	// warm the cache with this version
	_, err = versionCache.Get(version)

	if err != nil {
		return nil, errors.Wrapf(err, "getting default terraform version %s", defaultVersionStr)
	}

	builder := &CommandBuilder{
		defaultVersion: version,
		versionCache:   versionCache,
	}

	if usePluginCache {
		builder.terraformPluginCacheDir = cacheDir
	}

	asyncClient := &AsyncClient{
		projectCmdOutputHandler: projectCmdOutputHandler,
		commandBuilder:          builder,
	}

	// If tfeToken is set, we try to create a ~/.terraformrc file.
	if tfeToken != "" {
		home, err := homedir.Dir()
		if err != nil {
			return nil, errors.Wrap(err, "getting home dir")
		}
		if err := cloud.GenerateConfigFile(tfeToken, tfeHostname, home); err != nil {
			return nil, errors.Wrapf(err, "generating Terraform Cloud config file")
		}
	}
	return &DefaultClient{
		defaultVersion:   version,
		binDir:           binDir,
		downloader:       tfDownloader,
		downloadBaseURL:  tfDownloadURL,
		featureAllocator: featureAllocator,
		AsyncClient:      asyncClient,
		commandBuilder:   builder,
		versionCache:     versionCache,
	}, nil

}

func NewTestClient(
	log logging.SimpleLogging,
	binDir string,
	cacheDir string,
	tfeToken string,
	tfeHostname string,
	defaultVersionStr string,
	defaultVersionFlagName string,
	tfDownloadURL string,
	tfDownloader Downloader,
	usePluginCache bool,
	projectCmdOutputHandler handlers.ProjectCommandOutputHandler,
	featureAllocator feature.Allocator,
) (*DefaultClient, error) {
	return NewClientWithDefaultVersion(
		log,
		binDir,
		cacheDir,
		tfeToken,
		tfeHostname,
		defaultVersionStr,
		defaultVersionFlagName,
		tfDownloadURL,
		tfDownloader,
		usePluginCache,
		false,
		projectCmdOutputHandler,
		featureAllocator,
	)
}

// NewClient constructs a terraform client.
// tfeToken is an optional terraform enterprise token.
// defaultVersionStr is an optional default terraform version to use unless
// a specific version is set.
// defaultVersionFlagName is the name of the flag that sets the default terraform
// version.
// tfDownloader is used to download terraform versions.
// Will asynchronously download the required version if it doesn't exist already.
func NewClient(
	log logging.SimpleLogging,
	binDir string,
	cacheDir string,
	tfeToken string,
	tfeHostname string,
	defaultVersionStr string,
	defaultVersionFlagName string,
	tfDownloadURL string,
	tfDownloader Downloader,
	usePluginCache bool,
	projectCmdOutputHandler handlers.ProjectCommandOutputHandler,
	featureAllocator feature.Allocator,
) (*DefaultClient, error) {
	return NewClientWithDefaultVersion(
		log,
		binDir,
		cacheDir,
		tfeToken,
		tfeHostname,
		defaultVersionStr,
		defaultVersionFlagName,
		tfDownloadURL,
		tfDownloader,
		usePluginCache,
		true,
		projectCmdOutputHandler,
		featureAllocator,
	)
}

// Version returns the default version of Terraform we use if no other version
// is defined.
func (c *DefaultClient) DefaultVersion() *version.Version {
	return c.defaultVersion
}

// TerraformBinDir returns the directory where we download Terraform binaries.
func (c *DefaultClient) TerraformBinDir() string {
	return c.binDir
}

func (c *DefaultClient) EnsureVersion(log logging.SimpleLogging, v *version.Version) error {
	if v == nil {
		v = c.defaultVersion
	}

	_, err := c.versionCache.Get(v)

	if err != nil {
		return errors.Wrapf(err, "getting version %s", v)
	}

	return nil
}

// See Client.RunCommandWithVersion.
func (c *DefaultClient) RunCommandWithVersion(ctx models.ProjectCommandContext, path string, args []string, customEnvVars map[string]string, v *version.Version, workspace string) (string, error) {
	shouldAllocate, err := c.featureAllocator.ShouldAllocate(feature.LogStreaming, ctx.BaseRepo.FullName)

	if err != nil {
		ctx.Log.Err("unable to allocate for feature: %s, error: %s", feature.LogStreaming, err)
	}

	// if the feature is enabled, we use the async workflow else we default to the original sync workflow
	// Don't stream terraform show output to outCh
	if shouldAllocate && isAsyncEligibleCommand(args[0]) {
		outCh := c.RunCommandAsync(ctx, path, args, customEnvVars, v, workspace)

		var lines []string
		var err error
		for line := range outCh {
			if line.Err != nil {
				err = line.Err
				break
			}
			lines = append(lines, line.Line)
		}
		output := strings.Join(lines, "\n")

		// sanitize output by stripping out any ansi characters.
		output = ansi.Strip(output)
		return fmt.Sprintf("%s\n", output), err
	}

	cmd, err := c.commandBuilder.Build(v, workspace, path, args)
	if err != nil {
		return "", err
	}
	envVars := cmd.Env
	for key, val := range customEnvVars {
		envVars = append(envVars, fmt.Sprintf("%s=%s", key, val))
	}
	cmd.Env = envVars
	out, err := cmd.CombinedOutput()
	if err != nil {
		err = errors.Wrapf(err, "running %q in %q", cmd.String(), path)
		ctx.Log.Err(err.Error())
		return ansi.Strip(string(out)), err
	}
	ctx.Log.Info("successfully ran %q in %q", cmd.String(), path)

	return ansi.Strip(string(out)), nil
}

// Line represents a line that was output from a terraform command.
type Line struct {
	// Line is the contents of the line (without the newline).
	Line string
	// Err is set if there was an error.
	Err error
}

type versionLoader struct {
	downloader  Downloader
	downloadURL string
}

func (l *versionLoader) loadVersion(v *version.Version, destPath string) (runtime_models.FilePath, error) {
	urlPrefix := fmt.Sprintf("%s/terraform/%s/terraform_%s", l.downloadURL, v.String(), v.String())
	binURL := fmt.Sprintf("%s_%s_%s.zip", urlPrefix, runtime.GOOS, runtime.GOARCH)
	checksumURL := fmt.Sprintf("%s_SHA256SUMS", urlPrefix)
	fullSrcURL := fmt.Sprintf("%s?checksum=file:%s", binURL, checksumURL)
	if err := l.downloader.GetFile(destPath, fullSrcURL); err != nil {
		return runtime_models.LocalFilePath(""), errors.Wrapf(err, "downloading terraform version %s at %q", v.String(), fullSrcURL)
	}

	binPath := filepath.Join(destPath, "terraform")

	return runtime_models.LocalFilePath(binPath), nil

}

func isAsyncEligibleCommand(cmd string) bool {
	for _, validCmd := range LogStreamingValidCmds {
		if validCmd == cmd {
			return true
		}
	}
	return false
}

func getDefaultVersion(overrideVersion string, versionFlagName string) (*version.Version, error) {
	if overrideVersion != "" {
		v, err := version.NewVersion(overrideVersion)
		if err != nil {
			return nil, errors.Wrapf(err, "parsing version %s", overrideVersion)
		}

		return v, nil
	}

	// look for the binary directly on disk and query the version
	// we shouldn't really be doing this, but don't want to break existing clients.
	// we should be looking for an env var since we wont even be using this binary directly,
	// we'll be getting the version and downloading it to our cache.
	localPath, err := exec.LookPath("terraform")
	if err != nil {
		return nil, fmt.Errorf("terraform not found in $PATH. Set --%s or download terraform from https://www.terraform.io/downloads.html", versionFlagName)
	}

	return getVersion(localPath)
}

func getVersion(tfBinary string) (*version.Version, error) {
	versionOutBytes, err := exec.Command(tfBinary, "version").Output() // #nosec
	versionOutput := string(versionOutBytes)
	if err != nil {
		return nil, errors.Wrapf(err, "running terraform version: %s", versionOutput)
	}
	match := versionRegex.FindStringSubmatch(versionOutput)
	if len(match) <= 1 {
		return nil, fmt.Errorf("could not parse terraform version from %s", versionOutput)
	}
	return version.NewVersion(match[1])
}

type DefaultDownloader struct{}

// See go-getter.GetFile.
func (d *DefaultDownloader) GetFile(dst, src string, opts ...getter.ClientOption) error {
	return getter.GetFile(dst, src, opts...)
}

// See go-getter.GetFile.
func (d *DefaultDownloader) GetAny(dst, src string, opts ...getter.ClientOption) error {
	return getter.GetAny(dst, src, opts...)
}
