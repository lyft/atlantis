package link

import (
	"fmt"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/root"
	"net/url"
	"path"
)

type Builder struct{}

// BuildDownloadLinkFromArchive is a helper fxn that isolates the logic of modifying a GH archive link
// into source url that the go-getter library understand for downloading
func (b Builder) BuildDownloadLinkFromArchive(archiveURL *url.URL, root root.Root, repo github.Repo, revision string) string {
	// Add archive query parameter for getter library to extract archive
	queryParams := "archive=zip"
	token := archiveURL.Query().Get("token")
	if token != "" {
		queryParams += fmt.Sprintf("&token=%s", token)
	}
	archiveURL.RawQuery = queryParams

	// Append root subdirectory to path to trigger go-getter pkg to only copy the relevant files
	archiveName := fmt.Sprintf("%s-%s-%s", repo.Owner, repo.Name, revision)
	subDirPath := path.Join(archiveName, root.Path)

	archiveURL.Path = fmt.Sprintf("%s//%s", archiveURL.Path, subDirPath)
	return archiveURL.String()
}
