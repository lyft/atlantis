package source

import (
	"context"
	"github.com/runatlantis/atlantis/server/events/models"
)

type FileFetcher interface {
	GetModifiedFilesFromCommit(ctx context.Context, repo models.Repo, sha string, installationToken int64) ([]string, error)
}
