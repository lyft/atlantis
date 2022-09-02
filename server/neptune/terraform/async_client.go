package terraform

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"sync"

	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/runtime/cache"
	"github.com/runatlantis/atlantis/server/core/terraform"
	"github.com/runatlantis/atlantis/server/core/terraform/helpers"
	"github.com/runatlantis/atlantis/server/neptune/temporalworker/job"
)

// Setting the buffer size to 10mb
const BufioScannerBufferSize = 10 * 1024 * 1024

// versionRegex extracts the version from `terraform version` output.
//     Terraform v0.12.0-alpha4 (2c36829d3265661d8edbd5014de8090ea7e2a076)
//	   => 0.12.0-alpha4
//
//     Terraform v0.11.10
//	   => 0.11.10
var versionRegex = regexp.MustCompile("Terraform v(.*?)(\\s.*)?\n")

type clientAsync interface {
	RunCommandAsync(ctx context.Context, jobID string, path string, args []string, customEnvVars map[string]string, v *version.Version) <-chan helpers.Line
}

func NewAsyncClient(
	outputHandler *job.OutputHandler,
	binDir string,
	cacheDir string,
	defaultVersionStr string,
	defaultVersionFlagName string,
	tfDownloadURL string,
	tfDownloader terraform.Downloader,
	usePluginCache bool,
) (*AsyncClient, error) {
	version, err := GetDefaultVersion(defaultVersionStr, defaultVersionFlagName)
	if err != nil {
		return nil, errors.Wrapf(err, "getting default version")
	}

	loader := terraform.NewVersionLoader(&terraform.DefaultDownloader{}, tfDownloadURL)

	versionCache := cache.NewExecutionVersionLayeredLoadingCache(
		"terraform",
		binDir,
		loader.LoadVersion,
	)

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

	return &AsyncClient{
		StepOutputHandler: *outputHandler,
		CommandBuilder:    builder,
	}, nil

}

type AsyncClient struct {
	StepOutputHandler job.OutputHandler
	CommandBuilder    commandBuilder
}

func (c *AsyncClient) RunCommandAsync(ctx context.Context, jobID string, path string, args []string, customEnvVars map[string]string, v *version.Version) <-chan helpers.Line {
	outCh := make(chan helpers.Line)

	// We start a goroutine to do our work asynchronously and then immediately
	// return our channels.
	go func() {

		// Ensure we close our channels when we exit.
		defer func() {
			close(outCh)
		}()

		cmd, err := c.CommandBuilder.Build(v, path, args)
		if err != nil {
			// prjCtx.Log.ErrorContext(prjCtx.RequestCtx, err.Error())
			outCh <- helpers.Line{Err: err}
			return
		}
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()
		envVars := cmd.Env
		for key, val := range customEnvVars {
			envVars = append(envVars, fmt.Sprintf("%s=%s", key, val))
		}
		cmd.Env = envVars

		err = cmd.Start()
		if err != nil {
			err = errors.Wrapf(err, "running %q in %q", cmd.String(), path)
			// prjCtx.Log.ErrorContext(prjCtx.RequestCtx, err.Error())
			outCh <- helpers.Line{Err: err}
			return
		}

		// Use a waitgroup to block until our stdout/err copying is complete.
		wg := new(sync.WaitGroup)
		wg.Add(2)
		// Asynchronously copy from stdout/err to outCh.
		go func() {
			defer wg.Done()
			c.WriteOutput(stdout, outCh, jobID)
		}()
		go func() {
			defer wg.Done()
			c.WriteOutput(stderr, outCh, jobID)
		}()

		// Wait for our copying to complete. This *must* be done before
		// calling cmd.Wait(). (see https://github.com/golang/go/issues/19685)
		wg.Wait()

		// Wait for the command to complete.
		err = cmd.Wait()

		// We're done now. Send an error if there was one.
		if err != nil {
			err = errors.Wrapf(err, "running %q in %q", cmd.String(), path)
			// prjCtx.Log.ErrorContext(prjCtx.RequestCtx, err.Error())
			outCh <- helpers.Line{Err: err}
		} else {
			// prjCtx.Log.InfoContext(prjCtx.RequestCtx, fmt.Sprintf("successfully ran %q in %q", cmd.String(), path))
		}
	}()

	return outCh
}

func (c *AsyncClient) WriteOutput(stdReader io.ReadCloser, outCh chan helpers.Line, jobID string) {
	s := bufio.NewScanner(stdReader)
	buf := []byte{}
	s.Buffer(buf, BufioScannerBufferSize)

	for s.Scan() {
		message := s.Text()
		outCh <- helpers.Line{Line: message}
		c.StepOutputHandler.Send(jobID, message)
	}
}

func GetDefaultVersion(overrideVersion string, versionFlagName string) (*version.Version, error) {
	if overrideVersion != "" {
		v, err := version.NewVersion(overrideVersion)
		if err != nil {
			return nil, errors.Wrapf(err, "parsing version %s", overrideVersion)
		}

		return v, nil
	}

	// look for the binary directly on disk and query the version
	// we shouldn't really be doing this, but don't want to break existing clients.
	// this implementation assumes that versions in the format our cache assumes
	// and if thats the case we won't be redownloading the version of this binary to our cache
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
