package policy

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/legacy/core/runtime/cache"
	runtime_models "github.com/runatlantis/atlantis/server/legacy/core/runtime/models"
	"github.com/runatlantis/atlantis/server/legacy/core/terraform"
	"github.com/runatlantis/atlantis/server/logging"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	DefaultConftestVersionEnvKey = "DEFAULT_CONFTEST_VERSION"
	conftestBinaryName           = "conftest"
	conftestDownloadURLPrefix    = "https://github.com/open-policy-agent/conftest/releases/download/v"
	conftestArch                 = "x86_64"
)

type Arg struct {
	Param  string
	Option string
}

func (a Arg) build() []string {
	return []string{a.Option, a.Param}
}

func NewPolicyArg(parameter string) Arg {
	return Arg{
		Param:  parameter,
		Option: "-p",
	}
}

type ConftestTestCommandArgs struct {
	PolicyArgs []Arg
	ExtraArgs  []string
	InputFile  string
	Command    string
}

func (c ConftestTestCommandArgs) build() []string {
	// add the subcommand
	commandArgs := []string{c.Command, "test"}

	for _, a := range c.PolicyArgs {
		commandArgs = append(commandArgs, a.build()...)
	}

	// add hardcoded options
	commandArgs = append(commandArgs, c.InputFile, "--no-color")

	// add extra args provided through server config
	commandArgs = append(commandArgs, c.ExtraArgs...)

	return commandArgs
}

type ConfTestVersionDownloader struct {
	downloader terraform.Downloader
}

func (c ConfTestVersionDownloader) downloadConfTestVersion(v *version.Version, destPath string) (runtime_models.FilePath, error) {
	versionURLPrefix := fmt.Sprintf("%s%s", conftestDownloadURLPrefix, v.Original())

	// download binary in addition to checksum file
	binURL := fmt.Sprintf("%s/conftest_%s_%s_%s.tar.gz", versionURLPrefix, v.Original(), cases.Title(language.English).String(runtime.GOOS), conftestArch)
	checksumURL := fmt.Sprintf("%s/checksums.txt", versionURLPrefix)

	// underlying implementation uses go-getter so the URL is formatted as such.
	// i know i know, I'm assuming an interface implementation with my inputs.
	// realistically though the interface just exists for testing so ¯\_(ツ)_/¯
	fullSrcURL := fmt.Sprintf("%s?checksum=file:%s", binURL, checksumURL)

	if err := c.downloader.GetAny(destPath, fullSrcURL); err != nil {
		return runtime_models.LocalFilePath(""), errors.Wrapf(err, "downloading conftest version %s at %q", v.String(), fullSrcURL)
	}

	binPath := filepath.Join(destPath, "conftest")

	return runtime_models.LocalFilePath(binPath), nil
}

type ConfTestVersionEnsurer struct {
	VersionCache           cache.ExecutionVersionCache
	DefaultConftestVersion *version.Version
}

func NewConfTestVersionEnsurer(log logging.Logger, versionRootDir string, conftestDownloder terraform.Downloader) *ConfTestVersionEnsurer {
	downloader := ConfTestVersionDownloader{
		downloader: conftestDownloder,
	}
	version, err := getDefaultVersion()

	if err != nil {
		// conftest default versions are not essential to service startup so let's not block on it.
		log.Warn(fmt.Sprintf("failed to get default conftest version. Will attempt request scoped lazy loads %s", err.Error()))
	}

	versionCache := cache.NewExecutionVersionLayeredLoadingCache(
		conftestBinaryName,
		versionRootDir,
		downloader.downloadConfTestVersion,
	)

	return &ConfTestVersionEnsurer{
		VersionCache:           versionCache,
		DefaultConftestVersion: version,
	}
}

func (c *ConfTestVersionEnsurer) EnsureExecutorVersion(log logging.Logger, v *version.Version) (string, error) {
	// we have no information to proceed so fail hard
	if c.DefaultConftestVersion == nil && v == nil {
		return "", errors.New("no conftest version configured/specified")
	}

	var versionToRetrieve *version.Version

	if v == nil {
		versionToRetrieve = c.DefaultConftestVersion
	} else {
		versionToRetrieve = v
	}

	localPath, err := c.VersionCache.Get(versionToRetrieve)

	if err != nil {
		return "", err
	}

	return localPath, nil
}

func getDefaultVersion() (*version.Version, error) {
	// ensure version is not default version.
	// first check for the env var and if that doesn't exist use the local executable version
	defaultVersion, exists := os.LookupEnv(DefaultConftestVersionEnvKey)

	if !exists {
		return nil, fmt.Errorf("%s not set", DefaultConftestVersionEnvKey)
	}

	wrappedVersion, err := version.NewVersion(defaultVersion)

	if err != nil {
		return nil, errors.Wrapf(err, "wrapping version %s", defaultVersion)
	}
	return wrappedVersion, nil
}
