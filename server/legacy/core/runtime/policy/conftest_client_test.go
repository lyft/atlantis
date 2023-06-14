package policy

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hashicorp/go-version"
	. "github.com/petergtz/pegomock"
	"github.com/runatlantis/atlantis/server/legacy/core/runtime/cache/mocks"
	terraform_mocks "github.com/runatlantis/atlantis/server/legacy/core/terraform/mocks"
	"github.com/runatlantis/atlantis/server/logging"
	. "github.com/runatlantis/atlantis/testing"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func TestConfTestVersionDownloader(t *testing.T) {
	version, _ := version.NewVersion("0.25.0")
	destPath := "some/path"

	fullURL := fmt.Sprintf("https://github.com/open-policy-agent/conftest/releases/download/v0.25.0/conftest_0.25.0_%s_x86_64.tar.gz?checksum=file:https://github.com/open-policy-agent/conftest/releases/download/v0.25.0/checksums.txt", cases.Title(language.English).String(runtime.GOOS))

	RegisterMockTestingT(t)

	mockDownloader := terraform_mocks.NewMockDownloader()

	subject := ConfTestVersionDownloader{downloader: mockDownloader}

	t.Run("success", func(t *testing.T) {
		When(mockDownloader.GetFile(EqString(destPath), EqString(fullURL))).ThenReturn(nil)
		binPath, err := subject.downloadConfTestVersion(version, destPath)

		mockDownloader.VerifyWasCalledOnce().GetAny(EqString(destPath), EqString(fullURL))

		Ok(t, err)

		Assert(t, binPath.Resolve() == filepath.Join(destPath, "conftest"), "expected binpath")
	})

	t.Run("error", func(t *testing.T) {
		When(mockDownloader.GetAny(EqString(destPath), EqString(fullURL))).ThenReturn(errors.New("err"))
		_, err := subject.downloadConfTestVersion(version, destPath)

		Assert(t, err != nil, "err is expected")
	})
}

func TestEnsureExecutorVersion(t *testing.T) {
	defaultVersion, _ := version.NewVersion("1.0")
	expectedPath := "some/path"

	RegisterMockTestingT(t)

	mockCache := mocks.NewMockExecutionVersionCache()
	log := logging.NewNoopCtxLogger(t)

	t.Run("no specified version or default version", func(t *testing.T) {
		subject := &ConfTestVersionEnsurer{
			VersionCache: mockCache,
		}

		_, err := subject.EnsureExecutorVersion(log, nil)

		Assert(t, err != nil, "expected error finding version")
	})

	t.Run("use default version", func(t *testing.T) {
		subject := &ConfTestVersionEnsurer{
			VersionCache:           mockCache,
			DefaultConftestVersion: defaultVersion,
		}

		When(mockCache.Get(defaultVersion)).ThenReturn(expectedPath, nil)

		path, err := subject.EnsureExecutorVersion(log, nil)

		Ok(t, err)

		Assert(t, path == expectedPath, "path is expected")
	})

	t.Run("use specified version", func(t *testing.T) {
		subject := &ConfTestVersionEnsurer{
			VersionCache:           mockCache,
			DefaultConftestVersion: defaultVersion,
		}

		versionInput, _ := version.NewVersion("2.0")

		When(mockCache.Get(versionInput)).ThenReturn(expectedPath, nil)

		path, err := subject.EnsureExecutorVersion(log, versionInput)

		Ok(t, err)

		Assert(t, path == expectedPath, "path is expected")
	})

	t.Run("cache error", func(t *testing.T) {
		subject := &ConfTestVersionEnsurer{
			VersionCache:           mockCache,
			DefaultConftestVersion: defaultVersion,
		}

		versionInput, _ := version.NewVersion("2.0")

		When(mockCache.Get(versionInput)).ThenReturn(expectedPath, errors.New("some err"))

		_, err := subject.EnsureExecutorVersion(log, versionInput)

		Assert(t, err != nil, "path is expected")
	})
}
