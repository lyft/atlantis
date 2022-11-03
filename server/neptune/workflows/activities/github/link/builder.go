package link

import (
	"fmt"
	"net/url"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
)

type Builder struct{}

// BuildDownloadLinkFromArchive is a helper fxn that isolates the logic of modifying a GH archive link
// into source url that the go-getter library understand for downloading
func (b Builder) BuildDownloadLinkFromArchive(archiveURL *url.URL, root terraform.Root, repo github.Repo, revision string) string {
	// Add archive query parameter for getter library to extract archive
	queryParams := "archive=zip"
	token := archiveURL.Query().Get("token")
	if token != "" {
		queryParams += fmt.Sprintf("&token=%s", token)
	}
	archiveURL.RawQuery = queryParams
	return archiveURL.String()
}
