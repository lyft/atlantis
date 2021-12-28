//go:build wireinject
// +build wireinject

package server

import (
	"github.com/google/wire"
	stats "github.com/lyft/gostats"
	"github.com/runatlantis/atlantis/server/config"
	"github.com/runatlantis/atlantis/server/core/db"
	"github.com/runatlantis/atlantis/server/core/locking"
	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/handlers"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/lyft/scheduled"
)

func InitializeScheduledExecutorService(
	vcsClient vcs.Client,
	userConfig config.UserConfig,
	rawGithubClient *vcs.GithubClient,
	githubClient vcs.IGithubClient,
	logger logging.SimpleLogging,
	scope stats.Scope,
	projectCommandOutputHandler handlers.ProjectCommandOutputHandler,
	locker locking.Locker,
	boltdb *db.BoltDB,
	workingDir events.WorkingDir,
) (*scheduled.ExecutorService, error) {

	wire.Build(
		wire.Bind(new(events.EventParsing), new(*events.EventParser)),
		events.NewEventParser,
		scheduled.NewExecutorService,
		scheduled.NewStaleClosedPullExecutor,
		scheduled.NewStaleOpenPullExecutor,
		wire.Bind(new(handlers.ResourceCleaner), new(handlers.ProjectCommandOutputHandler)),
		wire.Bind(new(vcs.GithubPullRequestGetter), new(vcs.IGithubClient)),
		events.NewFileWorkDirIterator,
		wire.Bind(new(events.WorkDirIterator), new(*events.FileWorkDirIterator)),
	)

	return &scheduled.ExecutorService{}, nil
}
