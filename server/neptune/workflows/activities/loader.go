package activities

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/hashicorp/go-getter"
	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	runtime_models "github.com/runatlantis/atlantis/server/legacy/core/runtime/models"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const conftestDownloadURL = "https://github.com/open-policy-agent/conftest/releases/download/v"

var HashiGetAny = func(dst, src string) error {
	return getter.GetAny(dst, src)
}

type TFVersionLoader struct {
	downloadURL string
}

func NewTFVersionLoader(downloadURL string) *TFVersionLoader {
	return &TFVersionLoader{
		downloadURL: downloadURL,
	}
}

// TODO: migrate away from runtime_models.FilePath
func (t *TFVersionLoader) LoadVersion(v *version.Version, destPath string) (runtime_models.FilePath, error) {
	urlPrefix := fmt.Sprintf("%s/terraform/%s/terraform_%s", t.downloadURL, v.String(), v.String())
	binURL := fmt.Sprintf("%s_%s_%s.zip", urlPrefix, runtime.GOOS, runtime.GOARCH)
	checksumURL := fmt.Sprintf("%s_SHA256SUMS", urlPrefix)
	fullSrcURL := fmt.Sprintf("%s?checksum=file:%s", binURL, checksumURL)
	if err := HashiGetAny(destPath, fullSrcURL); err != nil {
		return runtime_models.LocalFilePath(""), errors.Wrapf(err, "downloading terraform version %s at %q", v.String(), fullSrcURL)
	}
	binPath := filepath.Join(destPath, "terraform")
	return runtime_models.LocalFilePath(binPath), nil
}

type ConftestVersionLoader struct{}

func (c *ConftestVersionLoader) LoadVersion(v *version.Version, destPath string) (runtime_models.FilePath, error) {
	urlPrefix := fmt.Sprintf("%s%s", conftestDownloadURL, v.Original())
	binURL := fmt.Sprintf("%s/conftest_%s_%s_x86_64.tar.gz", urlPrefix, v.Original(), cases.Title(language.English).String(runtime.GOOS))
	checksumURL := fmt.Sprintf("%s/checksums.txt", urlPrefix)
	fullSrcURL := fmt.Sprintf("%s?checksum=file:%s", binURL, checksumURL)
	if err := HashiGetAny(destPath, fullSrcURL); err != nil {
		return runtime_models.LocalFilePath(""), errors.Wrapf(err, "downloading conftest version %s at %q", v.String(), fullSrcURL)
	}
	binPath := filepath.Join(destPath, "conftest")
	return runtime_models.LocalFilePath(binPath), nil
}
