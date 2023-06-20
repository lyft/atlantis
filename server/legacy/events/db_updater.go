package events

import (
	"github.com/runatlantis/atlantis/server/legacy/core/db"
	"github.com/runatlantis/atlantis/server/legacy/events/command"
	"github.com/runatlantis/atlantis/server/models"
)

type DBUpdater struct {
	DB *db.BoltDB
}

func (c *DBUpdater) updateDB(_ *command.Context, pull models.PullRequest, results []command.ProjectResult) (models.PullStatus, error) {
	// Filter out results that errored due to the directory not existing. We
	// don't store these in the database because they would never be "apply-able"
	// and so the pull request would always have errors.
	var filtered []command.ProjectResult
	for _, r := range results {
		if _, ok := r.Error.(DirNotExistErr); ok {
			continue
		}
		filtered = append(filtered, r)
	}
	return c.DB.UpdatePullWithResults(pull, filtered)
}
